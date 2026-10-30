package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ovh/venom/internal/yamljson"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors"
	"github.com/ovh/venom/interpolate"
)

var (
	path []string
	v    *venom.Venom

	variables       []string
	secrets         []string
	format          = "xml" // Set the default value for formatFlag
	varFiles        []string
	outputDir       string
	libDir          string
	htmlReport      bool
	stopOnFailure   bool
	strictVariables bool
	verbose         int // Set the default value for verboseFlag
	parallel        = 1 // Set the default value for parallelFlag

	variablesFlag       *[]string
	formatFlag          *string
	varFilesFlag        *[]string
	outputDirFlag       *string
	libDirFlag          *string
	stopOnFailureFlag   *bool
	strictVariablesFlag *bool
	htmlReportFlag      *bool
	verboseFlag         *int
	parallelFlag        *int
)

func init() {
	formatFlag = Cmd.Flags().String("format", "xml", "--format:json, tap, xml, yaml")
	stopOnFailureFlag = Cmd.Flags().Bool("stop-on-failure", false, "Stop running Test Suite on first Test Case failure")
	strictVariablesFlag = Cmd.Flags().Bool("strict-variables", false, "Fail a step that names an unknown variable")
	htmlReportFlag = Cmd.Flags().Bool("html-report", false, "Generate HTML Report")
	verboseFlag = Cmd.Flags().CountP("verbose", "v", "verbose. -v (INFO level in venom.log file), -vv to very verbose (DEBUG level) and -vvv to very verbose with CPU Profiling")
	parallelFlag = Cmd.Flags().Int("parallel", 1, "Number of test files to run at the same time. 1 (default) runs them one after another")
	varFilesFlag = Cmd.Flags().StringSlice("var-from-file", []string{""}, "--var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary")
	variablesFlag = Cmd.Flags().StringArray("var", nil, "--var cds='cds -f config.json' --var cds2='cds -f config.json'")
	outputDirFlag = Cmd.PersistentFlags().String("output-dir", "", "Output Directory: create tests results file inside this directory")
	libDirFlag = Cmd.PersistentFlags().String("lib-dir", "", "Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib")
}

func initArgs(cmd *cobra.Command) {
	// command line flags overrides the configuration file.
	// Configuration file overrides the environment variables.
	if _, err := initFromEnv(os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		venom.OSExit(2)
	}

	if err := initFromConfigFile(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		venom.OSExit(2)
	}
	cmd.LocalFlags().VisitAll(initFromCommandArguments)
}

func initFromCommandArguments(f *pflag.Flag) {
	if !f.Changed {
		return
	}

	switch f.Name {
	case "format":
		setIfPresent(&format, formatFlag)
	case "stop-on-failure":
		setIfPresent(&stopOnFailure, stopOnFailureFlag)
	case "strict-variables":
		setIfPresent(&strictVariables, strictVariablesFlag)
	case "html-report":
		setIfPresent(&htmlReport, htmlReportFlag)
	case "output-dir":
		setIfPresent(&outputDir, outputDirFlag)
	case "lib-dir":
		setIfPresent(&libDir, libDirFlag)
	case "verbose":
		setIfPresent(&verbose, verboseFlag)
	case "parallel":
		setIfPresent(&parallel, parallelFlag)
	case "var-from-file":
		if varFilesFlag != nil {
			addVarFiles(*varFilesFlag)
		}
	case "var":
		if variablesFlag != nil {
			addVariables(*variablesFlag)
		}
	}
}

// setIfPresent copies *value into *option when value is set: a registered flag, or a key present in the config file.
func setIfPresent[T any](option *T, value *T) {
	if value != nil {
		*option = *value
	}
}

// addVarFiles appends to varFiles each file it does not list yet.
func addVarFiles(files []string) {
	for _, varFile := range files {
		if !isInArray(varFile, varFiles) {
			varFiles = append(varFiles, varFile)
		}
	}
}

// addVariables merges each name=value into variables, replacing an earlier value of the same name.
func addVariables(vars []string) {
	for _, variable := range vars {
		variables = mergeVariables(variable, variables)
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// homeDir returns the home folder: $HOME, else the account database (what go-homedir did through getent).
func homeDir() (string, error) {
	if home, err := os.UserHomeDir(); err == nil {
		return home, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if u.HomeDir == "" {
		return "", errors.New("home directory not found")
	}
	return u.HomeDir, nil
}

func initFromConfigFile() error {
	if fileExists(".venomrc") {
		fi, err := os.Open(".venomrc")
		if err != nil {
			return err
		}
		defer fi.Close()
		return initFromReaderConfigFile(fi)
	}

	home, err := homeDir()
	if err != nil {
		return err
	}
	if fileExists(filepath.Join(home, ".venomrc")) {
		fi, err := os.Open(filepath.Join(home, ".venomrc"))
		if err != nil {
			return err
		}
		defer fi.Close()
		return initFromReaderConfigFile(fi)
	}
	return nil
}

type ConfigFileData struct {
	Format          *string   `json:"format,omitempty" yaml:"format,omitempty"`
	LibDir          *string   `json:"lib_dir,omitempty" yaml:"lib_dir,omitempty"`
	OutputDir       *string   `json:"output_dir,omitempty" yaml:"output_dir,omitempty"`
	StopOnFailure   *bool     `json:"stop_on_failure,omitempty" yaml:"stop_on_failure,omitempty"`
	StrictVariables *bool     `json:"strict_variables,omitempty" yaml:"strict_variables,omitempty"`
	HtmlReport      *bool     `json:"html_report,omitempty" yaml:"html_report,omitempty"` //nolint:revive // exported field kept for module compatibility (plugins, library users, upstream merges)
	Variables       *[]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	Secrets         *[]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	VariablesFiles  *[]string `json:"variables_files,omitempty" yaml:"variables_files,omitempty"`
	Verbosity       *int      `json:"verbosity,omitempty" yaml:"verbosity,omitempty"`
	Parallel        *int      `json:"parallel,omitempty" yaml:"parallel,omitempty"`
}

// Configuration file overrides the environment variables.
func initFromReaderConfigFile(reader io.Reader) error {
	btes, err := io.ReadAll(reader)

	if err != nil && err != io.EOF {
		return err
	}

	var configFileData ConfigFileData
	if err := yamljson.Unmarshal(btes, &configFileData); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	applyConfigFile(configFileData)
	return nil
}

// applyConfigFile sets each option the config file names.
func applyConfigFile(configFileData ConfigFileData) {
	setIfPresent(&format, configFileData.Format)
	setIfPresent(&libDir, configFileData.LibDir)
	setIfPresent(&outputDir, configFileData.OutputDir)
	setIfPresent(&stopOnFailure, configFileData.StopOnFailure)
	setIfPresent(&strictVariables, configFileData.StrictVariables)
	setIfPresent(&htmlReport, configFileData.HtmlReport)
	if configFileData.Variables != nil {
		addVariables(*configFileData.Variables)
	}
	if configFileData.Secrets != nil {
		secrets = append(secrets, *configFileData.Secrets...)
	}
	if configFileData.VariablesFiles != nil {
		addVarFiles(*configFileData.VariablesFiles)
	}
	setIfPresent(&verbose, configFileData.Verbosity)
	setIfPresent(&parallel, configFileData.Parallel)
}

func mergeVariables(varToMerge string, existingVariables []string) []string {
	idx := strings.Index(varToMerge, "=")
	nameConfigFile := varToMerge[0:idx]
	for i, variable := range existingVariables {
		idx := strings.Index(variable, "=")
		if idx > 1 {
			nameEnv := variable[0:idx]
			if nameEnv == nameConfigFile {
				existingVariables[i] = varToMerge
				return existingVariables
			}
		}
	}
	existingVariables = append(existingVariables, varToMerge)
	return existingVariables
}

func isInArray(elt string, array []string) bool {
	for _, item := range array {
		if item == elt {
			return true
		}
	}
	return false
}

func initFromEnv(environ []string) ([]string, error) {
	if os.Getenv("VENOM_VAR") != "" {
		v := strings.Split(os.Getenv("VENOM_VAR"), " ")
		variables = v
	}

	if os.Getenv("VENOM_VAR_FROM_FILE") != "" {
		varFiles = strings.Split(os.Getenv("VENOM_VAR_FROM_FILE"), " ")
	}
	if os.Getenv("VENOM_FORMAT") != "" {
		format = os.Getenv("VENOM_FORMAT")
	}
	if err := initBoolsFromEnv(); err != nil {
		return nil, err
	}
	if os.Getenv("VENOM_LIB_DIR") != "" {
		libDir = os.Getenv("VENOM_LIB_DIR")
	}
	if os.Getenv("VENOM_OUTPUT_DIR") != "" {
		outputDir = os.Getenv("VENOM_OUTPUT_DIR")
	}
	if os.Getenv("VENOM_VERBOSE") != "" {
		v, err := strconv.ParseInt(os.Getenv("VENOM_VERBOSE"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for VENOM_VERBOSE, must be 1, 2 or 3")
		}
		v2 := int(v)
		verbose = v2
	}
	if os.Getenv("VENOM_PARALLEL") != "" {
		p, err := strconv.ParseInt(os.Getenv("VENOM_PARALLEL"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value for VENOM_PARALLEL, must be a whole number of 1 or more")
		}
		parallel = int(p)
	}

	addEnvVariables(environ)

	return variables, nil
}

// initBoolsFromEnv reads the boolean options of the environment. An invalid value fails after the option was
// overwritten with ParseBool's false (pinned by the tests).
func initBoolsFromEnv() error {
	for _, b := range []struct {
		name   string
		option *bool
	}{
		{"VENOM_STOP_ON_FAILURE", &stopOnFailure},
		{"VENOM_STRICT_VARIABLES", &strictVariables},
		{"VENOM_HTML_REPORT", &htmlReport},
	} {
		raw := os.Getenv(b.name)
		if raw == "" {
			continue
		}
		var err error
		*b.option, err = strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid value for %s", b.name)
		}
	}
	return nil
}

// addEnvVariables appends one name=value to variables for each VENOM_VAR_<name> entry of environ.
func addEnvVariables(environ []string) {
	for _, env := range environ {
		if strings.HasPrefix(env, "VENOM_VAR_") {
			tuple := strings.SplitN(env, "=", 2)
			k := strings.TrimPrefix(tuple[0], "VENOM_VAR_")
			variables = append(variables, fmt.Sprintf("%s=%s", k, tuple[1]))
		}
	}
}

func displayArg(ctx context.Context) {
	venom.Debug(ctx, "option format=%v", format)
	venom.Debug(ctx, "option libDir=%v", libDir)
	venom.Debug(ctx, "option outputDir=%v", outputDir)
	venom.Debug(ctx, "option stopOnFailure=%v", stopOnFailure)
	venom.Debug(ctx, "option strictVariables=%v", strictVariables)
	venom.Debug(ctx, "option htmlReport=%v", htmlReport)
	venom.Debug(ctx, "option varFiles=%v", strings.Join(varFiles, " "))
	venom.Debug(ctx, "option verbose=%v", verbose)
	venom.Debug(ctx, "option parallel=%v", parallel)
}

// validateArgs checks the option combinations that cannot work, so the run fails at startup with
// something a reader can act on instead of misbehaving later (#2020).
func validateArgs() error {
	if parallel < 1 {
		return fmt.Errorf("invalid value %d for --parallel: it must be 1 or more (1 runs the test files one after another)", parallel)
	}
	if parallel > 1 && verbose >= 3 {
		// pprof's CPU profiler is process-global: one profile can be running at a time, and
		// stopping it stops it for everyone. Per-file profiles cannot be produced concurrently.
		return fmt.Errorf("--parallel %d cannot be combined with -vvv: CPU profiling is process-wide, "+
			"so it cannot produce one profile per test file running at the same time. "+
			"Use -vv, or run with --parallel 1", parallel)
	}
	return nil
}

// Cmd run
var Cmd = &cobra.Command{
	Use:   "run",
	Short: "Run Tests",
	Example: `  Run all testsuites containing in files ending with *.yml or *.yaml: venom run
  Run a single testsuite: venom run mytestfile.yml
  Run a single testsuite and export the result in JSON format in test/ folder: venom run mytestfile.yml --format=json --output-dir=test
  Run a single testsuite and export the result in XML and HTML formats in test/ folder: venom run mytestfile.yml --format=xml --output-dir=test --html-report
  Run a single testsuite and specify a variable: venom run mytestfile.yml --var="foo=bar"
  Run a single testsuite and load all variables from a file: venom run mytestfile.yml --var-from-file variables.yaml
  Run all testsuites containing in files ending with *.yml or *.yaml with verbosity: VENOM_VERBOSE=2 venom run
  Run four test files at the same time: venom run --parallel 4
  
  Notice that variables initialized with -var-from-file argument can be overrided with -var argument
  
  More info: https://github.com/ovh/venom`,
	Long: `run integration tests`,
	PreRun: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			path = append(path, ".")
		} else {
			path = args[0:]
		}

		v = venom.New()
		for name, executorFunc := range executors.Registry {
			v.RegisterExecutorBuiltin(name, executorFunc())
		}
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		initArgs(cmd)

		if err := validateArgs(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			venom.OSExit(2)
		}

		v.OutputDir = outputDir
		v.LibDir = libDir
		v.OutputFormat = format
		v.StopOnFailure = stopOnFailure
		v.StrictVariables = strictVariables
		v.HtmlReport = htmlReport
		v.Verbose = verbose
		v.Parallel = parallel

		if err := v.InitLogger(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			venom.OSExit(2)
		}

		// There is deliberately no run-level CPU profile here. venom profiles each test file
		// individually (runTestSuite), which is the original -vvv feature and the more useful
		// shape — it says which suite is slow. Go's CPU profiler is process-wide, so a second
		// profile started here could only fight with those: it left the first file's profile at
		// zero bytes and truncated its own when that file's deferred StopCPUProfile fired (#2024).
		//
		// The memory profile that used to be written here never worked either: it was produced by
		// a deferred WriteTo, and this function ends in venom.OSExit, so the defer never ran and
		// the file was always empty.
		if verbose >= 2 {
			displayArg(context.Background())
		}

		readers := []io.Reader{}
		for _, f := range varFiles {
			if f == "" {
				continue
			}
			fi, err := os.Open(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unable to open var-from-file %s: %v\n", f, err)
				venom.OSExit(2)
			}
			defer fi.Close()
			readers = append(readers, fi)
		}

		mapvars, err := readInitialVariables(context.Background(), variables, readers, os.Environ())
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			venom.OSExit(2)
		}
		v.AddVariables(mapvars)
		v.AddRunSecrets(secrets)

		if err := v.Parse(context.Background(), path); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			venom.OSExit(2)
		}

		if err := v.Process(context.Background(), path); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			venom.OSExit(2)
		}

		if err := v.OutputResult(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			venom.OSExit(2)
		}

		if v.Tests.Status == venom.StatusPass {
			fmt.Fprintf(os.Stdout, "final status: %v\n", venom.Green(v.Tests.Status))
			venom.OSExit(0)
		}
		fmt.Fprintf(os.Stdout, "final status: %v\n", venom.Red(v.Tests.Status))
		venom.OSExit(2)

		return nil
	},
}

func readInitialVariables(ctx context.Context, argsVars []string, argVarsFiles []io.Reader, _ []string) (map[string]interface{}, error) {
	cast := func(vS string) interface{} {
		var v interface{}
		_ = yamljson.Unmarshal([]byte(vS), &v) //nolint:errcheck // best-effort type cast of a --var value: an unparseable value stays nil, as before
		return v
	}

	result := map[string]interface{}{}

	for _, r := range argVarsFiles {
		tmpResult := map[string]interface{}{}
		btes, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}

		stemp, err := interpolate.Do(string(btes), nil)
		if err != nil {
			return nil, errors.Wrap(err, "unable to interpolate file")
		}

		if err := yamljson.Unmarshal([]byte(stemp), &tmpResult); err != nil {
			return nil, errors.Wrap(err, "unable to unmarshal file")
		}

		for k, v := range tmpResult {
			result[k] = v
			venom.Debug(ctx, "Adding variable from vars-files %s=%s", k, v)
		}
	}

	for _, arg := range argsVars {
		if arg == "" {
			continue
		}
		tuple := strings.SplitN(arg, "=", 2)
		if len(tuple) < 2 {
			return nil, fmt.Errorf("invalid variable declaration: %v", arg)
		}
		stemp, err := interpolate.Do(tuple[1], nil)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to interpolate arg %s", arg)
		}
		result[tuple[0]] = cast(stemp)
		venom.Debug(ctx, "Adding variable from vars arg %s=%s", tuple[0], result[tuple[0]])
	}

	return result, nil
}
