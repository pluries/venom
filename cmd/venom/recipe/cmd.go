// Package recipe is the "venom recipe" command: static checks of web and mobile recipe files.
package recipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/ovh/venom"
	"github.com/ovh/venom/internal/recipecheck"
)

const (
	formatText = "text"
	formatJSON = "json"

	exitFindings = 1
	exitUnusable = 2
)

// Cmd is the recipe command.
var Cmd = &cobra.Command{
	Use:   "recipe",
	Short: "Check recipes: venom suites written from an exploration of a web or mobile app",
}

var (
	format   string
	specsDir string
	coverage string
)

var checkCmd = &cobra.Command{
	Use:   "check <path>...",
	Short: "Check recipe files against the rules RC01 to RC11: venom recipe check [--format text|json] <path>...",
	Run: func(cmd *cobra.Command, args []string) {
		if code := runCheck(cmd.OutOrStdout(), cmd.ErrOrStderr(), args); code != 0 {
			venom.OSExit(code)
		}
	},
}

func init() {
	checkCmd.Flags().StringVar(&format, "format", formatText, "output format: text or json")
	checkCmd.Flags().StringVar(&specsDir, "specs-dir", "docs/specs", "where the spec named by a suite is looked up")
	checkCmd.Flags().StringVar(&coverage, "coverage", "", "list the scenarios of this spec id with the testcases that prove them")
	Cmd.AddCommand(checkCmd)
}

// runCheck runs the check and returns the exit code: 0 nothing found, 1 findings, 2 unusable input.
func runCheck(out, errOut io.Writer, paths []string) int {
	if format != formatText && format != formatJSON {
		fmt.Fprintf(errOut, "unknown --format %q: use text or json\n", format)
		return exitUnusable
	}
	report, err := recipecheck.Check(paths, recipecheck.Options{SpecsDir: specsDir, Coverage: coverage})
	if len(report.Findings) > 0 {
		printReport(out, report)
	}
	switch {
	case errors.Is(err, recipecheck.ErrUnusable):
		fmt.Fprintln(errOut, err)
		return exitUnusable
	case err != nil:
		fmt.Fprintln(errOut, err)
		return exitUnusable
	case len(report.Findings) > 0:
		printCoverage(out, report)
		return exitFindings
	}
	printSummary(out, report)
	printCoverage(out, report)
	return 0
}

// printCoverage lists the scenarios of the --coverage spec after the findings; the JSON report carries them itself.
func printCoverage(out io.Writer, report recipecheck.Report) {
	if format == formatJSON {
		return
	}
	if coverage != "" && len(report.Coverage) == 0 {
		fmt.Fprintf(out, "the spec %s lists no scenario\n", coverage)
	}
	for _, c := range report.Coverage {
		if !c.Proven {
			fmt.Fprintf(out, "%s not proven\n", c.Scenario)
			continue
		}
		for _, tc := range c.Testcases {
			fmt.Fprintf(out, "%s proven: %s / %s\n", c.Scenario, tc.File, tc.Testcase)
		}
	}
}

func printSummary(out io.Writer, report recipecheck.Report) {
	if format == formatJSON {
		printReport(out, report)
		return
	}
	fmt.Fprintf(out, "%d files read, no finding\n", report.Files)
}

// where names the suite and, when the finding has one, its testcase.
func where(f recipecheck.Finding) string {
	if f.Testcase == "" {
		return f.Suite
	}
	return f.Suite + " / " + f.Testcase
}

func textLine(f recipecheck.Finding) string {
	return fmt.Sprintf("%s:%d: %s %s (%s)", f.File, f.Line, f.Rule, f.Message, where(f))
}

// sortFindings orders findings by file, line and rule, on a copy.
func sortFindings(findings []recipecheck.Finding) []recipecheck.Finding {
	sorted := append([]recipecheck.Finding(nil), findings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	return sorted
}

func printReport(out io.Writer, report recipecheck.Report) {
	report.Findings = sortFindings(report.Findings)
	if format == formatJSON {
		if report.Findings == nil {
			report.Findings = []recipecheck.Finding{}
		}
		b, _ := json.Marshal(report) //nolint:errcheck // a struct of strings and ints always marshals
		fmt.Fprintln(out, string(b))
		return
	}
	for _, f := range report.Findings {
		fmt.Fprintln(out, textLine(f))
	}
	fmt.Fprintf(out, "%d finding(s) in %d file(s)\n", len(report.Findings), report.Files)
}
