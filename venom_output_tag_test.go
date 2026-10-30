package venom

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// capture collects everything written through PrintFunc. The engine serialises tagged writes under
// v.printMu, so the untagged case is the only one that could race here — and it is only used from
// a single goroutine in these tests.
type capture struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (c *capture) printf(format string, a ...interface{}) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Fprintf(&c.buf, format, a...)
}

func (c *capture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// taggedVenom returns a Venom whose output is captured, with `files` test files registered so the
// tag width is computed as it would be in a real run.
func taggedVenom(t *testing.T, parallel int, filenames ...string) (*Venom, *capture) {
	t.Helper()
	v := New()
	v.Parallel = parallel
	c := &capture{}
	v.PrintFunc = c.printf
	for _, f := range filenames {
		v.Tests.TestSuites = append(v.Tests.TestSuites, TestSuite{Filename: f})
	}
	return v, c
}

// tagRe matches the file tag at the start of a line. venom colours its output and those escape
// sequences contain '[' too, so the tag has to be matched by shape, not by counting brackets.
var tagRe = regexp.MustCompile(`\[[A-Za-z0-9._-]+ *\]`)

// tagsOf returns the file tags found in a line. More than one means two files tore it.
func tagsOf(line string) []string {
	out := []string{}
	for _, m := range tagRe.FindAllString(line, -1) {
		out = append(out, strings.TrimSpace(m[1:len(m)-1]))
	}
	return out
}

// TestSuiteOutIsTransparentWhenSequential pins AC-1 at the unit level: at --parallel 1 the printer
// adds nothing — no tag, no line buffering, the exact bytes venom has always written.
func TestSuiteOutIsTransparentWhenSequential(t *testing.T) {
	for _, parallel := range []int{0, 1} {
		t.Run(fmt.Sprintf("parallel=%d", parallel), func(t *testing.T) {
			v, c := taggedVenom(t, parallel, "exec.yml")
			out := v.newSuiteOut(&v.Tests.TestSuites[0])

			// A line built from two calls, as runTestCases does: the name, then the status.
			out.Print(" \t• %s", "my case")
			out.Println(" %s", "PASS")

			require.Equal(t, " \t• my case PASS\n", c.String())
			// Nothing may be held back: sequential output must appear as it is written.
			out.Flush()
			require.Equal(t, " \t• my case PASS\n", c.String())
		})
	}
}

// TestSuiteOutTagsEveryPhysicalLine pins that a single call carrying several lines — a systemout
// dump, say — is tagged throughout. An untagged continuation line is exactly the line a reader
// cannot attribute to a file.
func TestSuiteOutTagsEveryPhysicalLine(t *testing.T) {
	v, c := taggedVenom(t, 4, "exec.yml")
	out := v.newSuiteOut(&v.Tests.TestSuites[0])

	out.Println("first\nsecond\nthird")
	out.Flush()

	require.Equal(t, "[exec.yml] first\n[exec.yml] second\n[exec.yml] third\n", c.String())
}

// TestSuiteOutHoldsAPartialLine pins the fix for the defect the first working build showed: venom
// composes one line from several calls, so emitting each call as it arrives let another file's
// output land mid-line ("[s4.yml] • c4[s2.yml] • suite 2").
func TestSuiteOutHoldsAPartialLine(t *testing.T) {
	v, c := taggedVenom(t, 4, "exec.yml")
	out := v.newSuiteOut(&v.Tests.TestSuites[0])

	out.Print(" \t• %s", "my case")
	require.Empty(t, c.String(), "an unterminated line must not be emitted yet")

	out.Println(" %s", "PASS")
	require.Equal(t, "[exec.yml]  \t• my case PASS\n", c.String())
}

// TestSuiteOutFlushEmitsAnUnterminatedLine pins that a file ending mid-line still reports it —
// otherwise an aborted run loses the test case it died on.
func TestSuiteOutFlushEmitsAnUnterminatedLine(t *testing.T) {
	v, c := taggedVenom(t, 4, "exec.yml")
	out := v.newSuiteOut(&v.Tests.TestSuites[0])

	out.Print(" \t• %s", "interrupted")
	require.Empty(t, c.String())

	out.Flush()
	require.Equal(t, "[exec.yml]  \t• interrupted\n", c.String())

	out.Flush() // flushing twice must not duplicate it
	require.Equal(t, "[exec.yml]  \t• interrupted\n", c.String())
}

// TestSuiteOutPadsTagsToAColumn pins that the tags line up, which is what makes a parallel run
// scannable by eye rather than only by grep.
func TestSuiteOutPadsTagsToAColumn(t *testing.T) {
	v, c := taggedVenom(t, 4, "a.yml", "a_much_longer_name.yml")

	for i := range v.Tests.TestSuites {
		out := v.newSuiteOut(&v.Tests.TestSuites[i])
		out.Println("line")
	}

	lines := strings.Split(strings.TrimSuffix(c.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, "[a.yml                 ] line", lines[0])
	require.Equal(t, "[a_much_longer_name.yml] line", lines[1])
	require.Equal(t, len(lines[0]), len(lines[1]), "tags must form a column")
}

// TestSuiteOutNeverTearsALineUnderConcurrency is the AC-5 guard, and the reason the printer holds
// a partial line at all. Several files print at once, each composing its lines from two calls;
// every emitted line must carry exactly one tag and belong to exactly one file.
//
// Run with -race.
func TestSuiteOutNeverTearsALineUnderConcurrency(t *testing.T) {
	const files, linesPerFile = 8, 50

	names := make([]string, files)
	for i := range names {
		names[i] = fmt.Sprintf("file%02d.yml", i)
	}
	v, c := taggedVenom(t, files, names...)

	var wg sync.WaitGroup
	for i := range v.Tests.TestSuites {
		wg.Add(1)
		go func(ts *TestSuite) {
			defer wg.Done()
			out := v.newSuiteOut(ts)
			defer out.Flush()
			for j := 0; j < linesPerFile; j++ {
				// Two calls per line, as the engine does.
				out.Print(" \t• case-%02d", j)
				out.Println(" %s", StatusPass)
			}
		}(&v.Tests.TestSuites[i])
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(c.String(), "\n"), "\n")
	require.Len(t, lines, files*linesPerFile, "every line must be emitted exactly once")

	perFile := map[string]int{}
	for _, l := range lines {
		tags := tagsOf(l)
		require.Len(t, tags, 1, "line carries %d tags, so two files tore it: %q", len(tags), l)
		require.Contains(t, l, " PASS", "line %q was cut before its status", l)
		perFile[tags[0]]++
	}
	require.Len(t, perFile, files)
	for _, n := range names {
		require.Equal(t, linesPerFile, perFile[n], "file %s lost or duplicated lines", n)
	}
}

// TestProcessTagsEveryLineOfAParallelRun is the end-to-end guard for AC-5: a real Process run at
// --parallel 4, with the output captured as a terminal would receive it. It exercises the actual
// print path — runTestCases and printTestStepResult — rather than the printer alone.
//
// **verbose=1 is not optional here.** The per-step prints — the one in runTestSteps and the nine in
// printTestStepResult — are all gated on `v.Verbose >= 1`, and they are the only ones that reach the
// printer through v.out(tc). At verbose 0 every asserted line comes from runTestCases' own local
// printer instead, so the tc-borne sink, which is the whole mechanism this task adds, is exercised
// by nothing: deleting `tc.out = out` would leave the suite green while a real `-v --parallel 4` run
// printed every step line untagged and, because the fallback printer has an empty prefix and so
// skips printMu, torn. Found in code review.
func TestProcessTagsEveryLineOfAParallelRun(t *testing.T) {
	for _, verbose := range []int{0, 1} {
		t.Run(fmt.Sprintf("verbose=%d", verbose), func(t *testing.T) {
			InitTestLogger(t)

			const files = 6
			v := New()
			v.Parallel = 4
			v.Verbose = verbose
			v.RegisterExecutorBuiltin("sleeper", sleeperExecutor{})
			c := &capture{}
			v.PrintFunc = c.printf

			v.Tests.TestSuites = waitingSuites(t, files, 15*time.Millisecond)
			for i := range v.Tests.TestSuites {
				v.Tests.TestSuites[i].Filename = fmt.Sprintf("suite-%02d.yml", i)
			}

			require.NoError(t, v.Process(context.Background(), nil))

			lines := strings.Split(strings.TrimSuffix(c.String(), "\n"), "\n")
			require.NotEmpty(t, lines)

			seen := map[string]bool{}
			for _, l := range lines {
				tags := tagsOf(l)
				require.NotEmpty(t, tags,
					"every line of a parallel run must carry its file's tag, got %q", l)
				require.Len(t, tags, 1, "line carries %d tags, so two files tore it: %q", len(tags), l)
				require.True(t, strings.HasPrefix(l, "["), "the tag must lead the line, got %q", l)
				seen[tags[0]] = true
			}
			require.Len(t, seen, files, "every file should have printed something")

			if verbose >= 1 {
				// Prove the step lines really were produced, so this subtest cannot pass by
				// asserting on the same lines the verbose=0 one sees.
				require.Greater(t, len(lines), files*2,
					"verbose 1 must emit the per-step lines that go through v.out(tc)")
			}
		})
	}
}

// TestProcessDoesNotTagASequentialRun is the AC-1 counterpart: with the option off, not a single
// tag appears anywhere.
func TestProcessDoesNotTagASequentialRun(t *testing.T) {
	InitTestLogger(t)

	v := New()
	v.Parallel = 1
	v.RegisterExecutorBuiltin("sleeper", sleeperExecutor{})
	c := &capture{}
	v.PrintFunc = c.printf

	v.Tests.TestSuites = waitingSuites(t, 3, 0)
	for i := range v.Tests.TestSuites {
		v.Tests.TestSuites[i].Filename = fmt.Sprintf("suite-%02d.yml", i)
	}

	require.NoError(t, v.Process(context.Background(), nil))

	out := c.String()
	require.NotEmpty(t, out)
	require.NotContains(t, out, "[suite-", "a sequential run must print no tag at all")
}

// TestProcessTagsAFailedAndASkippedFile pins that the files a stopped run did not finish are still
// reported, and still attributable — the drain case #2020 introduced.
func TestProcessTagsAFailedAndASkippedFile(t *testing.T) {
	InitTestLogger(t)

	v := New()
	v.Parallel = 2
	v.StopOnFailure = true
	v.RegisterExecutorBuiltin("sleeper", sleeperExecutor{})
	c := &capture{}
	v.PrintFunc = c.printf

	v.Tests.TestSuites = append([]TestSuite{failingSuite(t, "fails")}, waitingSuites(t, 4, 10*time.Millisecond)...)
	for i := range v.Tests.TestSuites {
		v.Tests.TestSuites[i].Filename = fmt.Sprintf("suite-%02d.yml", i)
	}

	require.NoError(t, v.Process(context.Background(), nil))

	out := c.String()
	require.Contains(t, out, "[suite-00.yml]", "the failing file must still be attributable")
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		require.Len(t, tagsOf(l), 1, "torn line in a stopped run: %q", l)
	}
}
