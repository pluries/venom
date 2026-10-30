package venom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/mattn/go-zglob"
	"github.com/ovh/venom/internal/yamljson"
	yamlv3 "go.yaml.in/yaml/v3"

	"github.com/ovh/venom/interpolate"
	"github.com/pkg/errors"
)

// testCaseLineInfo holds source line numbers for a single test case.
type testCaseLineInfo struct {
	TestCaseLine   int     // line of the testcase mapping node
	StepLines      []int   // line of each step mapping node
	AssertionLines [][]int // [stepIdx][assertIdx] line numbers
}

// extractLineNumbers parses raw YAML bytes using go.yaml.in/yaml/v3 to extract
// source line numbers for testcases, steps, and assertions.
func extractLineNumbers(content []byte) []testCaseLineInfo {
	testcasesNode := testcasesSequence(content)
	if testcasesNode == nil {
		return nil
	}

	result := make([]testCaseLineInfo, 0, len(testcasesNode.Content))
	for _, tcNode := range testcasesNode.Content {
		result = append(result, testCaseLines(tcNode))
	}
	return result
}

// testcasesSequence returns the "testcases" sequence of a suite file, or nil when the file has none.
func testcasesSequence(content []byte) *yamlv3.Node {
	var doc yamlv3.Node
	if err := yamlv3.Unmarshal(content, &doc); err != nil {
		return nil
	}

	// doc is a DocumentNode containing the root MappingNode
	if doc.Kind != yamlv3.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yamlv3.MappingNode {
		return nil
	}

	testcasesNode := mappingValue(root, "testcases")
	if testcasesNode == nil || testcasesNode.Kind != yamlv3.SequenceNode {
		return nil
	}
	return testcasesNode
}

// testCaseLines returns the line of a testcase node, of its steps and of their assertions.
func testCaseLines(tcNode *yamlv3.Node) testCaseLineInfo {
	if tcNode.Kind != yamlv3.MappingNode {
		return testCaseLineInfo{}
	}

	info := testCaseLineInfo{
		TestCaseLine: tcNode.Line,
	}

	stepsNode := mappingValue(tcNode, "steps")
	if stepsNode == nil || stepsNode.Kind != yamlv3.SequenceNode {
		return info
	}

	info.StepLines = make([]int, len(stepsNode.Content))
	info.AssertionLines = make([][]int, len(stepsNode.Content))
	for stepIdx, stepNode := range stepsNode.Content {
		info.StepLines[stepIdx] = stepNode.Line
		info.AssertionLines[stepIdx] = assertionLines(stepNode)
	}
	return info
}

// assertionLines returns the line of each assertion of a step node, or nil when it has no assertion sequence.
func assertionLines(stepNode *yamlv3.Node) []int {
	if stepNode.Kind != yamlv3.MappingNode {
		return nil
	}

	assertionsNode := mappingValue(stepNode, "assertions")
	if assertionsNode == nil || assertionsNode.Kind != yamlv3.SequenceNode {
		return nil
	}

	lines := make([]int, len(assertionsNode.Content))
	for aIdx, aNode := range assertionsNode.Content {
		lines[aIdx] = aNode.Line
	}
	return lines
}

// mappingValue returns the value node of the first key named key in a mapping node, or nil.
func mappingValue(mapping *yamlv3.Node, key string) *yamlv3.Node {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func getFilesPath(path []string) ([]string, error) {
	filePaths := make([]string, 0)

	for _, p := range path {
		p = strings.TrimSpace(p)

		// No need to check err on os.Stat: a glob pattern like ./test/*.yml is a legitimate, common
		// input here and always makes Stat return an error; only a real directory path is used below.
		fileInfo, _ := os.Stat(p) //nolint:errcheck // p may be a glob pattern (not a real path), which always errors here by design; only a successful, IsDir() result is acted on
		if fileInfo != nil && fileInfo.IsDir() {
			// check if *.yml or *.yaml files exists in the path
			p = p + string(os.PathSeparator) + "*.y*ml"
		}

		fpaths, err := zglob.Glob(p)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading files on path %q", path)
		}

		for _, fp := range fpaths {
			switch ext := filepath.Ext(fp); ext {
			case ".yml", ".yaml":
				filePaths = append(filePaths, fp)
			}
		}
	}

	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no YAML (*.yml or *.yaml) file found or defined")
	}
	return uniq(filePaths), nil
}

func uniq(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func (v *Venom) readFiles(ctx context.Context, filesPath []string) (err error) {
	for _, filePath := range filesPath {
		ts, err := v.readFile(ctx, filePath)
		if err != nil {
			return err
		}
		v.Tests.TestSuites = append(v.Tests.TestSuites, ts)
	}
	return nil
}

// readFile reads, interpolates and unmarshals one suite file into a TestSuite.
func (v *Venom) readFile(ctx context.Context, filePath string) (TestSuite, error) {
	Debug(ctx, "Reading %v", filePath)
	btes, err := os.ReadFile(filePath)
	if err != nil {
		return TestSuite{}, errors.Wrapf(err, "unable to read file %q", filePath)
	}

	varCloned, err := v.suiteVars(ctx, filePath, btes)
	if err != nil {
		return TestSuite{}, err
	}

	var vars map[string]string
	if len(varCloned) > 0 {
		vars, err = DumpStringPreserveCase(varCloned)
		if err != nil {
			return TestSuite{}, errors.Wrapf(err, "unable to parse variables")
		}
	}

	content, err := interpolate.Do(string(btes), vars)
	if err != nil {
		return TestSuite{}, err
	}

	var testSuiteInput TestSuiteInput
	if err := yamljson.Unmarshal([]byte(content), &testSuiteInput); err != nil {
		Error(context.Background(), "file %q is not valid YAML after interpolation; its content is not logged as it may hold secrets", filePath)
		return TestSuite{}, errors.Wrapf(err, "error while unmarshal file %q", filePath)
	}

	ts := TestSuite{
		Name:        testSuiteInput.Name,
		Description: testSuiteInput.Description,
		TestCases:   testCasesWithLines(testSuiteInput.TestCases, btes),
		Vars:        testSuiteInput.Vars,
		Secrets:     mergeSecretNames(testSuiteInput.Secrets, v.runSecrets),
	}
	Info(ctx, "Has %d Secrets", len(ts.Secrets))

	if err := setSuiteLocation(&ts, filePath); err != nil {
		return TestSuite{}, err
	}
	ts.Vars = varCloned
	addSuiteVars(&ts)
	return ts, nil
}

// mergeSecretNames returns the secret names of a suite: its own names, then the run-level ones (AddRunSecrets) it does
// not declare already, each once. Without a run-level name the suite's own list is returned untouched.
func mergeSecretNames(own, run []string) []string {
	if len(run) == 0 {
		return own
	}
	merged := make([]string, 0, len(own)+len(run))
	merged = append(merged, own...)
	for _, name := range run {
		if !slices.Contains(merged, name) {
			merged = append(merged, name)
		}
	}
	return merged
}

// suiteVars returns the run vars completed by the vars the suite file declares.
func (v *Venom) suiteVars(ctx context.Context, filePath string, btes []byte) (H, error) {
	varCloned := v.variables.Clone()

	fromPartial, err := getVarFromPartialYML(ctx, btes)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get vars from file %q", filePath)
	}

	var varsFromPartial map[string]string
	if len(fromPartial) > 0 {
		varsFromPartial, err = DumpStringPreserveCase(fromPartial)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse variables")
		}
	}

	// we take default vars from the testsuite, only if it's not already is global vars
	for k, value := range varsFromPartial {
		if k == "" {
			continue
		}
		if _, ok := varCloned[k]; !ok || (varCloned[k] == "{}" && varCloned["__Len__"] == "0") {
			// we interpolate the value of vars here, to do it only once per ts
			valueInterpolated, err := interpolateSuiteVar(k, value, varsFromPartial)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to parse variable %q", k)
			}
			varCloned.Add(k, valueInterpolated)
		}
	}
	return varCloned, nil
}

// testCasesWithLines builds the suite's testcases with their source lines, read before interpolation.
func testCasesWithLines(inputs []TestCaseInput, btes []byte) []TestCase {
	testCases := make([]TestCase, len(inputs))
	lineInfos := extractLineNumbers(btes)

	for i := range inputs {
		testCases[i] = TestCase{
			TestCaseInput: inputs[i],
		}
		if i < len(lineInfos) {
			testCases[i].SourceLine = lineInfos[i].TestCaseLine
			testCases[i].StepSourceLines = lineInfos[i].StepLines
			testCases[i].AssertionSourceLines = lineInfos[i].AssertionLines
		}
	}
	return testCases
}

// setSuiteLocation sets the suite's working directory and file names from its path.
func setSuiteLocation(ts *TestSuite, filePath string) error {
	var err error
	// Default workdir is testsuite directory
	ts.WorkDir, err = filepath.Abs(filepath.Dir(filePath))
	if runtime.GOOS == "windows" {
		// Replace backslashes with forward slashes for Windows
		ts.WorkDir = strings.ReplaceAll(ts.WorkDir, "\\", "/")
	}
	if err != nil {
		return errors.Wrapf(err, "Unable to get testsuite's working directory")
	}

	// ../foo/a.yml
	ts.Filepath = filePath
	// a
	ts.ShortName = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	// a.yml
	ts.Filename = filepath.Base(filePath)
	return nil
}

// addSuiteVars adds the venom.testsuite.* and time vars to the suite's vars.
func addSuiteVars(ts *TestSuite) {
	ts.Vars.Add("venom.testsuite.workdir", ts.WorkDir)
	ts.Vars.Add("venom.testsuite.name", ts.Name)
	ts.Vars.Add("venom.testsuite.shortName", ts.ShortName)
	ts.Vars.Add("venom.testsuite.filename", ts.Filename)
	ts.Vars.Add("venom.testsuite.filepath", ts.Filepath)
	ts.Vars.Add("venom.datetime", time.Now().Format(time.RFC3339))
	ts.Vars.Add("venom.timestamp", fmt.Sprintf("%d", time.Now().Unix()))
}
