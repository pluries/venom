package venom

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// benchNestedExecutors returns the user executors of a chain depth levels deep: level 1 runs one fake step and calls
// level 2, and so on; the last level runs one fake step only. Each level has an input, reads it, captures the fake's
// output into a variable and returns it as its output, the shape of tests/lib/user_executor_A.yml and _B.yml.
func benchNestedExecutors(depth int) []UserExecutor {
	uxs := make([]UserExecutor, 0, depth)
	for level := 1; level <= depth; level++ {
		var b strings.Builder
		fmt.Fprintf(&b, "executor: nest%dlevel%d\n", depth, level)
		b.WriteString("input:\n  foo: default\n")
		b.WriteString("steps:\n")
		b.WriteString("- type: fake\n  label: '{{.input.foo}}'\n  out: 'level {{.input.foo}}'\n")
		b.WriteString("  vars:\n    said:\n      from: result.systemout\n")
		if level < depth {
			fmt.Fprintf(&b, "- type: nest%dlevel%d\n  foo: '{{.input.foo}}_%d'\n", depth, level+1, level+1)
		}
		b.WriteString("output:\n  said: '{{.said}}'\n")
		uxs = append(uxs, stepUserExecutor(b.String()))
	}
	return uxs
}

// BenchmarkRunUserExecutorNested measures venom's own cost of one custom executor call as the chain of custom executors
// it starts gets deeper (#1980). The inner steps run a fake executor that does nothing, so the time is the engine's:
// flattening the variables, substituting and parsing the executor file, running the inner steps, building the output.
//
// The suite variables are the per-step shape of BenchmarkDumpStringPreserveCase: Small has no nested value, Large holds
// one 110 KB, three-level-deep value (#2017 found the value's size, not the number of variables, drives the flatten).
// Secret runs the call as a suite that declares one secret does: venom's log functions then print each argument to
// redact it, whether or not the log level writes the line (redactLogArgs, log.go).
func BenchmarkRunUserExecutorNested(b *testing.B) {
	// a run without -v logs warnings only (InitLogger); the benchmark does the same, and writes nowhere
	InitTestLogger(nil)
	logger.Logger.SetLevel(logrus.WarnLevel)
	logger.Logger.SetOutput(io.Discard)
	secretCtxs := []struct {
		name string
		ctx  context.Context
	}{
		{"NoSecret", context.Background()},
		{"Secret", context.WithValue(context.Background(), ContextKey("secrets"), []string{"bench-secret-value"})},
	}
	sizes := []struct {
		name    string
		vars    H
		noSetup bool
	}{
		{"Small", realisticStepVars(nil), false},
		{"Large_110KB", realisticStepVars(nestedJSONValue(3, 12)), false},
		{"SuiteVar_110KB", benchSuiteVars(b, nestedJSONValue(3, 12)), false},
		// the fake executor has a Setup, which is given the whole scope of the test case; most builtins have none (#2475)
		{"SuiteVar_110KB_NoSetup", benchSuiteVars(b, nestedJSONValue(3, 12)), true},
	}
	for _, sc := range secretCtxs {
		for _, size := range sizes {
			for depth := 1; depth <= 3; depth++ {
				benchRunUserExecutorNested(sc.ctx, b, fmt.Sprintf("%s/%s/depth%d", sc.name, size.name, depth), size.vars, depth, size.noSetup, false)
			}
		}
	}
}

// BenchmarkRunUserExecutorNestedCold is BenchmarkRunUserExecutorNested for the first step of a test case (#2488): every
// iteration prepares the suite scope again, as a new root test case does, so the time includes the one-time build the warm
// benchmark pays once and then never again.
func BenchmarkRunUserExecutorNestedCold(b *testing.B) {
	InitTestLogger(nil)
	logger.Logger.SetLevel(logrus.WarnLevel)
	logger.Logger.SetOutput(io.Discard)
	sizes := []struct {
		name    string
		vars    H
		noSetup bool
	}{
		{"Small", realisticStepVars(nil), false},
		{"SuiteVar_110KB_NoSetup", benchSuiteVars(b, nestedJSONValue(3, 12)), true},
		{"SuiteVar_110KB", benchSuiteVars(b, nestedJSONValue(3, 12)), false},
	}
	for _, size := range sizes {
		for depth := 1; depth <= 3; depth++ {
			benchRunUserExecutorNested(context.Background(), b, fmt.Sprintf("NoSecret/%s/depth%d", size.name, depth), size.vars, depth, size.noSetup, true)
		}
	}
}

// BenchmarkPrepareSuite and BenchmarkLowerOver measure the two one-time builds of a suiteScope on their own (#2488): the
// flattened, escaped and indexed scope every step reads, and the dump a custom executor with a Setup starts from, whose
// index is derived from the first's (#2496), so it is timed on a scope that is already prepared.
func BenchmarkPrepareSuite(b *testing.B) {
	for _, size := range benchBuildSizes(b) {
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if prepareSuite(size.vars) == nil && len(size.vars) > 0 {
					b.Fatal("the suite is not prepared")
				}
			}
		})
	}
}

func BenchmarkLowerOver(b *testing.B) {
	for _, size := range benchBuildSizes(b) {
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				// lowerOver derives its index from the prepared scope's, which the first step of a test case has built
				b.StopTimer()
				suite := newSuiteScope(size.vars)
				suite.prepared()
				b.StartTimer()
				if suite.lowerOver() == nil {
					b.Fatal("the suite variables cannot be dumped")
				}
			}
		})
	}
}

type benchBuildSize struct {
	name string
	vars H
}

func benchBuildSizes(b *testing.B) []benchBuildSize {
	b.Helper()
	return []benchBuildSize{
		{"Small", realisticStepVars(nil)},
		{"SuiteVar_110KB", benchSuiteVars(b, nestedJSONValue(3, 12))},
	}
}

// benchSuiteVars returns the suite variables a test case of a real run receives when the suite declares value as a
// variable: initTestSuiteVars keeps the nested value and adds every one of its flattened keys next to it, as strings.
// Large_110KB holds the value alone, the shape of a variable a step captured, which is not how a suite variable reaches
// a custom executor.
func benchSuiteVars(b *testing.B, value interface{}) H {
	b.Helper()
	ts := &TestSuite{Name: "bench", Vars: realisticStepVars(value)}
	if err := New().initTestSuiteVars(ts); err != nil {
		b.Fatal(err)
	}
	return ts.Vars
}

// benchRunUserExecutorNested runs one custom executor call per iteration. The suite scope is prepared once for all of them,
// or again at each iteration when cold is set.
func benchRunUserExecutorNested(ctx context.Context, b *testing.B, name string, suiteVars H, depth int, noSetup, cold bool) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		v := New()
		v.PrintFunc = func(string, ...interface{}) (int, error) { return 0, nil }
		var fake Executor = stepNewFake("fake")
		if noSetup {
			fake = benchNoSetup{fake}
		}
		v.RegisterExecutorBuiltin("fake", fake)
		uxs := benchNestedExecutors(depth)
		for _, ux := range uxs {
			if err := v.RegisterExecutorUser(ux.Executor, ux); err != nil {
				b.Fatal(err)
			}
		}
		runner := newExecutorRunner(uxs[0], uxs[0].Executor, "user", 0, nil, 0, 0, nil)
		step := TestStep{"type": uxs[0].Executor, "foo": "bar"}

		suite := newSuiteScope(suiteVars)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if cold {
				suite = newSuiteScope(suiteVars)
			}
			benchCallOnce(ctx, b, v, runner, step, suiteVars, suite)
		}
	})
}

// benchCallOnce runs one custom executor call on a new test case that holds suite, and fails the benchmark when it errs.
func benchCallOnce(ctx context.Context, b *testing.B, v *Venom, runner ExecutorRunner, step TestStep, suiteVars H, suite *suiteScope) {
	b.Helper()
	tcIn := &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}, originalName: "outer", TestSuiteVars: suiteVars, suite: suite}
	tsIn := &TestStepResult{}
	if _, err := v.RunUserExecutor(ctx, runner, tcIn, tsIn, step); err != nil {
		b.Fatal(err)
	}
	if len(tsIn.Errors) > 0 {
		b.Fatalf("inner step failed: %v", tsIn.Errors)
	}
}

// benchNoSetup runs the executor it wraps, and hides its Setup and TearDown: an executor that has none is given nothing.
type benchNoSetup struct {
	Executor
}
