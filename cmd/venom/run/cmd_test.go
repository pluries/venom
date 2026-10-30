package run

import (
	"context"
	"errors"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

func Test_readInitialVariables(t *testing.T) {
	venom.InitTestLogger(t)
	type args struct {
		argsVars     []string
		argVarsFiles []io.Reader
		env          []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "from args",
			args: args{
				argsVars: []string{`a=1`, `b="B"`, `c=[1,2,3]`},
			},
			want: map[string]interface{}{
				"a": 1.0,
				"b": "B",
				"c": []interface{}{1.0, 2.0, 3.0},
			},
		},
		{
			name: "from args",
			args: args{
				argsVars: []string{`db.dsn="user=test password=test dbname=yo host=localhost port=1234 sslmode=disable"`},
			},
			want: map[string]interface{}{
				"db.dsn": "user=test password=test dbname=yo host=localhost port=1234 sslmode=disable",
			},
		},
		{
			name: "from readers",
			args: args{
				argVarsFiles: []io.Reader{
					strings.NewReader(`
a: 1
b: B
c:
  - 1
  - 2
  - 3`),
				},
			},
			want: map[string]interface{}{
				"a": 1.0,
				"b": "B",
				"c": []interface{}{1.0, 2.0, 3.0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readInitialVariables(context.TODO(), tt.args.argsVars, tt.args.argVarsFiles, tt.args.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("readInitialVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.EqualValues(t, tt.want, got)
		})
	}
}

func Test_mergeVariables(t *testing.T) {
	ma := mergeVariables("aa=bb", []string{"cc=dd", "ee=ff"})
	require.Equal(t, 3, len(ma))

	mb := mergeVariables("aa=bb", []string{"aa=dd"})
	require.Equal(t, 1, len(mb))

	mc := mergeVariables("aa=bb=dd", []string{"aa=dd"})
	require.Equal(t, 1, len(mc))

	md := mergeVariables("aa=bb=dd", []string{"cc=dd"})
	require.Equal(t, 2, len(md))
}

func Test_initFromEnv(t *testing.T) {
	env := []string{`VENOM_VAR_a=1`, `VENOM_VAR_b="B"`, `VENOM_VAR_c=[1,2,3]`, `VENOM_VAR_d="e=f"`}
	found, err := initFromEnv(env)
	require.NoError(t, err)
	require.Equal(t, 4, len(found))
	var nb int
	for i := range found {
		switch found[i] {
		case "a=1":
			nb++
		case "b=B":
			nb++
		case "c=[1,2,3]":
			nb++
		case "d=e=f":
			nb++
		}
	}
}

func Test_validateArgs_parallel(t *testing.T) {
	// The package-level options are global, so each case sets both and restores them after.
	defer func(p, v int) { parallel, verbose = p, v }(parallel, verbose)

	tests := []struct {
		name        string
		parallel    int
		verbose     int
		wantErr     bool
		errContains string
	}{
		{name: "the default", parallel: 1, verbose: 0},
		{name: "several workers", parallel: 8, verbose: 0},
		{name: "several workers with -vv", parallel: 4, verbose: 2},
		{name: "one worker with -vvv is still fine", parallel: 1, verbose: 3},
		{name: "zero is refused", parallel: 0, verbose: 0, wantErr: true, errContains: "must be 1 or more"},
		{name: "negative is refused", parallel: -3, verbose: 0, wantErr: true, errContains: "must be 1 or more"},
		{
			name: "-vvv with several workers is refused", parallel: 2, verbose: 3,
			wantErr: true, errContains: "CPU profiling is process-wide",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parallel, verbose = tt.parallel, tt.verbose
			err := validateArgs()
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func Test_initFromEnv_parallel(t *testing.T) {
	defer func(p int) { parallel = p }(parallel)

	t.Setenv("VENOM_PARALLEL", "6")
	_, err := initFromEnv([]string{})
	require.NoError(t, err)
	require.Equal(t, 6, parallel)

	t.Setenv("VENOM_PARALLEL", "not-a-number")
	_, err = initFromEnv([]string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "VENOM_PARALLEL")
}

func Test_initFromReaderConfigFile_parallel(t *testing.T) {
	defer func(p int) { parallel = p }(parallel)

	require.NoError(t, initFromReaderConfigFile(strings.NewReader("parallel: 3\n")))
	require.Equal(t, 3, parallel)
}

// options is a snapshot of the package-level run options the three init functions write.
type options struct {
	format          string
	stopOnFailure   bool
	strictVariables bool
	htmlReport      bool
	outputDir       string
	libDir          string
	verbose         int
	parallel        int
	varFiles        []string
	variables       []string
	secrets         []string
}

func currentOptions() options {
	return options{
		format: format, stopOnFailure: stopOnFailure, strictVariables: strictVariables, htmlReport: htmlReport, outputDir: outputDir, libDir: libDir,
		verbose: verbose, parallel: parallel, varFiles: varFiles, variables: variables, secrets: secrets,
	}
}

func setOptions(o options) {
	format, stopOnFailure, htmlReport, outputDir, libDir = o.format, o.stopOnFailure, o.htmlReport, o.outputDir, o.libDir
	strictVariables = o.strictVariables
	verbose, parallel = o.verbose, o.parallel
	varFiles, variables, secrets = slices.Clone(o.varFiles), slices.Clone(o.variables), slices.Clone(o.secrets)
}

// defaultOptions are the values the package declares before any flag, file or environment variable is read.
var defaultOptions = options{format: "xml", parallel: 1}

// venomEnv lists the single-value environment variables initFromEnv reads with os.Getenv.
var venomEnv = []string{
	"VENOM_VAR", "VENOM_VAR_FROM_FILE", "VENOM_FORMAT", "VENOM_STOP_ON_FAILURE", "VENOM_STRICT_VARIABLES", "VENOM_HTML_REPORT",
	"VENOM_LIB_DIR", "VENOM_OUTPUT_DIR", "VENOM_VERBOSE", "VENOM_PARALLEL",
}

// isolateOptions saves every package-level option and flag pointer, starts the test from `start`, blanks the VENOM_*
// environment of the developer's shell, and restores everything when the test ends, so the cases do not depend on the
// order they run in.
func isolateOptions(t *testing.T, start options) {
	t.Helper()
	saved := currentOptions()
	saved.varFiles, saved.variables, saved.secrets = slices.Clone(varFiles), slices.Clone(variables), slices.Clone(secrets)
	fFormat, fStop, fHTML, fOut, fLib, fVerbose, fParallel := formatFlag, stopOnFailureFlag, htmlReportFlag, outputDirFlag, libDirFlag, verboseFlag, parallelFlag
	fVarFiles, fVars, fStrict := varFilesFlag, variablesFlag, strictVariablesFlag
	t.Cleanup(func() {
		setOptions(saved)
		formatFlag, stopOnFailureFlag, htmlReportFlag, outputDirFlag, libDirFlag, verboseFlag, parallelFlag = fFormat, fStop, fHTML, fOut, fLib, fVerbose, fParallel
		varFilesFlag, variablesFlag, strictVariablesFlag = fVarFiles, fVars, fStrict
	})
	for _, k := range venomEnv {
		t.Setenv(k, "")
	}
	setOptions(start)
}

func ptr[T any](v T) *T { return &v }

// TestInitFromCommandArgumentsCopiesChangedFlags characterises initFromCommandArguments before #2031 splits it: a flag
// the user changed is copied into its package-level option, anything else leaves every option as it was.
func TestInitFromCommandArgumentsCopiesChangedFlags(t *testing.T) {
	tests := []struct {
		name    string
		flag    pflag.Flag
		start   options
		prepare func() // points the flag variables at the values the user typed
		want    options
	}{
		{
			name: "an unchanged flag is ignored", flag: pflag.Flag{Name: "format"}, start: defaultOptions,
			prepare: func() { formatFlag = ptr("json") }, want: defaultOptions,
		},
		{
			name: "a changed flag unknown to the function is ignored", flag: pflag.Flag{Name: "no-such-flag", Changed: true}, start: defaultOptions,
			prepare: func() {}, want: defaultOptions,
		},
		{
			name: "a changed flag whose variable is nil is ignored", flag: pflag.Flag{Name: "format", Changed: true}, start: defaultOptions,
			prepare: func() { formatFlag = nil }, want: defaultOptions,
		},
		{
			name: "format", flag: pflag.Flag{Name: "format", Changed: true}, start: defaultOptions,
			prepare: func() { formatFlag = ptr("json") }, want: options{format: "json", parallel: 1},
		},
		{
			name: "stop-on-failure", flag: pflag.Flag{Name: "stop-on-failure", Changed: true}, start: defaultOptions,
			prepare: func() { stopOnFailureFlag = ptr(true) }, want: options{format: "xml", parallel: 1, stopOnFailure: true},
		},
		{
			name: "stop-on-failure set back to false", flag: pflag.Flag{Name: "stop-on-failure", Changed: true}, start: options{format: "xml", parallel: 1, stopOnFailure: true},
			prepare: func() { stopOnFailureFlag = ptr(false) }, want: defaultOptions,
		},
		{
			name: "strict-variables", flag: pflag.Flag{Name: "strict-variables", Changed: true}, start: defaultOptions,
			prepare: func() { strictVariablesFlag = ptr(true) }, want: options{format: "xml", parallel: 1, strictVariables: true},
		},
		{
			name: "html-report", flag: pflag.Flag{Name: "html-report", Changed: true}, start: defaultOptions,
			prepare: func() { htmlReportFlag = ptr(true) }, want: options{format: "xml", parallel: 1, htmlReport: true},
		},
		{
			name: "output-dir", flag: pflag.Flag{Name: "output-dir", Changed: true}, start: defaultOptions,
			prepare: func() { outputDirFlag = ptr("out") }, want: options{format: "xml", parallel: 1, outputDir: "out"},
		},
		{
			name: "lib-dir", flag: pflag.Flag{Name: "lib-dir", Changed: true}, start: defaultOptions,
			prepare: func() { libDirFlag = ptr("lib:other") }, want: options{format: "xml", parallel: 1, libDir: "lib:other"},
		},
		{
			name: "verbose", flag: pflag.Flag{Name: "verbose", Changed: true}, start: defaultOptions,
			prepare: func() { verboseFlag = ptr(2) }, want: options{format: "xml", parallel: 1, verbose: 2},
		},
		{
			name: "parallel is copied without validation", flag: pflag.Flag{Name: "parallel", Changed: true}, start: defaultOptions,
			prepare: func() { parallelFlag = ptr(0) }, want: options{format: "xml", parallel: 0},
		},
		{
			name: "var-from-file appends the files not listed yet, once each", flag: pflag.Flag{Name: "var-from-file", Changed: true},
			start:   options{format: "xml", parallel: 1, varFiles: []string{"a.yml"}},
			prepare: func() { varFilesFlag = ptr([]string{"a.yml", "b.yml", "b.yml"}) },
			want:    options{format: "xml", parallel: 1, varFiles: []string{"a.yml", "b.yml"}},
		},
		{
			// quirk: mergeVariables only replaces names longer than one character (idx > 1), so "b" is appended twice
			name: "var replaces a variable of the same name, except one-letter names", flag: pflag.Flag{Name: "var", Changed: true},
			start:   options{format: "xml", parallel: 1, variables: []string{"aa=1", "b=1"}},
			prepare: func() { variablesFlag = ptr([]string{"aa=2", "b=2", "c=x=y"}) },
			want:    options{format: "xml", parallel: 1, variables: []string{"aa=2", "b=1", "b=2", "c=x=y"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateOptions(t, tt.start)
			tt.prepare()
			flag := tt.flag
			initFromCommandArguments(&flag)
			assert.Equal(t, tt.want, currentOptions())
		})
	}
}

// TestInitFromReaderConfigFile characterises initFromReaderConfigFile before #2031 splits it: which key sets which
// option, how lists merge with what is already set, and the exact errors. The file is decoded before any option is
// written, so a failing file leaves every option as it was.
func TestInitFromReaderConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		reader  io.Reader
		start   options
		want    options
		wantErr string
	}{
		{
			name: "every scalar key",
			reader: strings.NewReader("format: json\nlib_dir: lib\noutput_dir: out\nstop_on_failure: true\nhtml_report: true\n" +
				"verbosity: 3\nparallel: 4\n"),
			start: defaultOptions,
			want:  options{format: "json", libDir: "lib", outputDir: "out", stopOnFailure: true, htmlReport: true, verbose: 3, parallel: 4},
		},
		{
			name:   "strict_variables",
			reader: strings.NewReader("strict_variables: true\n"),
			start:  defaultOptions,
			want:   options{format: "xml", parallel: 1, strictVariables: true},
		},
		{
			name:   "a key set to its zero value overrides the current value",
			reader: strings.NewReader("format: \"\"\nstop_on_failure: false\nparallel: 0\n"),
			start:  options{format: "xml", parallel: 2, stopOnFailure: true},
			want:   options{format: "", parallel: 0, stopOnFailure: false},
		},
		{
			name:   "an absent key leaves its option unchanged",
			reader: strings.NewReader("format: tap\n"),
			start:  options{format: "xml", parallel: 3, outputDir: "kept"},
			want:   options{format: "tap", parallel: 3, outputDir: "kept"},
		},
		{
			name:   "JSON syntax is accepted",
			reader: strings.NewReader(`{"format": "tap"}`),
			start:  defaultOptions,
			want:   options{format: "tap", parallel: 1},
		},
		{
			name:   "an unknown key is ignored",
			reader: strings.NewReader("unknown_key: 1\n"),
			start:  defaultOptions,
			want:   defaultOptions,
		},
		{
			// quirk: mergeVariables only replaces names longer than one character (idx > 1)
			name:   "variables merge by name, except one-letter names",
			reader: strings.NewReader("variables:\n  - aa=2\n  - b=2\n  - c=3\n"),
			start:  options{format: "xml", parallel: 1, variables: []string{"aa=1", "b=1"}},
			want:   options{format: "xml", parallel: 1, variables: []string{"aa=2", "b=1", "b=2", "c=3"}},
		},
		{
			name:   "secrets are appended without deduplication",
			reader: strings.NewReader("secrets:\n  - token\n  - other\n"),
			start:  options{format: "xml", parallel: 1, secrets: []string{"token"}},
			want:   options{format: "xml", parallel: 1, secrets: []string{"token", "token", "other"}},
		},
		{
			name:   "variables_files appends the files not listed yet, once each",
			reader: strings.NewReader("variables_files:\n  - a.yml\n  - b.yml\n  - b.yml\n"),
			start:  options{format: "xml", parallel: 1, varFiles: []string{"a.yml"}},
			want:   options{format: "xml", parallel: 1, varFiles: []string{"a.yml", "b.yml"}},
		},
		{
			name: "an empty file changes nothing", reader: strings.NewReader(""), start: defaultOptions, want: defaultOptions,
		},
		{
			name: "a file of spaces changes nothing", reader: strings.NewReader("   "), start: defaultOptions, want: defaultOptions,
		},
		{
			name: "a file of comments changes nothing", reader: strings.NewReader("# only a comment\n# another\n"),
			start: defaultOptions, want: defaultOptions,
		},
		{
			name: "a read error is returned as it is", reader: iotest.ErrReader(errors.New("disk on fire")), start: defaultOptions,
			want: defaultOptions, wantErr: "disk on fire",
		},
		{
			name: "malformed YAML", reader: strings.NewReader("format: json\n  bad: indent\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "error converting YAML to JSON: yaml: line 2: mapping values are not allowed in this context",
		},
		{
			name: "a key with no name", reader: strings.NewReader(":\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "error converting YAML to JSON: yaml: did not find expected key",
		},
		{
			name: "a list instead of a mapping", reader: strings.NewReader("- a\n- b\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal array into Go value of type run.ConfigFileData",
		},
		{
			name: "a list where a string is expected", reader: strings.NewReader("format: [a]\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal array into Go struct field .format of type string",
		},
		{
			name: "a word where a number is expected, after a valid key", reader: strings.NewReader("format: json\nparallel: abc\n"),
			start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal string into Go struct field .parallel of type int",
		},
		{
			name: "a decimal where an integer is expected", reader: strings.NewReader("verbosity: 1.5\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal number 1.5 into Go struct field .verbosity of type int",
		},
		{
			name: "a word where a boolean is expected", reader: strings.NewReader("stop_on_failure: maybe\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal string into Go struct field .stop_on_failure of type bool",
		},
		{
			name: "yes is not a boolean", reader: strings.NewReader("html_report: yes\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal string into Go struct field .html_report of type bool",
		},
		{
			name: "a string where a list is expected", reader: strings.NewReader("variables: a=1\n"), start: defaultOptions, want: defaultOptions,
			wantErr: "json: cannot unmarshal string into Go struct field .variables of type []string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateOptions(t, tt.start)
			err := initFromReaderConfigFile(tt.reader)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, currentOptions())
		})
	}
}

// TestInitFromEnv characterises initFromEnv before #2031 splits it. The single-value variables are read from the
// process environment (os.Getenv), not from the environ argument; only the VENOM_VAR_<name> entries come from environ.
func TestInitFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		environ  []string
		start    options
		want     options
		wantVars []string // the returned slice
		wantErr  string
	}{
		{
			name: "nothing set changes nothing", start: options{format: "xml", parallel: 1, variables: []string{"a=1"}},
			want: options{format: "xml", parallel: 1, variables: []string{"a=1"}}, wantVars: []string{"a=1"},
		},
		{
			name: "VENOM_VAR is split on spaces and replaces the current variables", env: map[string]string{"VENOM_VAR": "a=1 b=2"},
			start: options{format: "xml", parallel: 1, variables: []string{"z=0"}},
			want:  options{format: "xml", parallel: 1, variables: []string{"a=1", "b=2"}}, wantVars: []string{"a=1", "b=2"},
		},
		{
			// quirk: the split ignores quotes
			name: "VENOM_VAR splits a quoted value holding a space", env: map[string]string{"VENOM_VAR": "a='x y'"}, start: defaultOptions,
			want: options{format: "xml", parallel: 1, variables: []string{"a='x", "y'"}}, wantVars: []string{"a='x", "y'"},
		},
		{
			name: "VENOM_VAR_FROM_FILE is split on spaces and replaces the current files", env: map[string]string{"VENOM_VAR_FROM_FILE": "a.yml b.yml"},
			start: options{format: "xml", parallel: 1, varFiles: []string{"old.yml"}},
			want:  options{format: "xml", parallel: 1, varFiles: []string{"a.yml", "b.yml"}},
		},
		{
			name: "format, lib and output directories",
			env:  map[string]string{"VENOM_FORMAT": "json", "VENOM_LIB_DIR": "lib", "VENOM_OUTPUT_DIR": "out"}, start: defaultOptions,
			want: options{format: "json", libDir: "lib", outputDir: "out", parallel: 1},
		},
		{
			name: "booleans accept strconv.ParseBool spellings", env: map[string]string{"VENOM_STOP_ON_FAILURE": "1", "VENOM_HTML_REPORT": "TRUE"},
			start: defaultOptions, want: options{format: "xml", parallel: 1, stopOnFailure: true, htmlReport: true},
		},
		{
			name: "a false boolean overrides a true option", env: map[string]string{"VENOM_STOP_ON_FAILURE": "false", "VENOM_HTML_REPORT": "0"},
			start: options{format: "xml", parallel: 1, stopOnFailure: true, htmlReport: true}, want: defaultOptions,
		},
		{
			name: "VENOM_STRICT_VARIABLES is read like the other booleans", env: map[string]string{"VENOM_STRICT_VARIABLES": "true"},
			start: defaultOptions, want: options{format: "xml", parallel: 1, strictVariables: true},
		},
		{
			name: "an invalid VENOM_STRICT_VARIABLES", env: map[string]string{"VENOM_STRICT_VARIABLES": "yes"},
			start: defaultOptions, want: defaultOptions, wantErr: "invalid value for VENOM_STRICT_VARIABLES",
		},
		{
			// quirk: the message says 1, 2 or 3 but any integer is taken
			name: "VENOM_VERBOSE takes any integer", env: map[string]string{"VENOM_VERBOSE": "7"}, start: defaultOptions,
			want: options{format: "xml", parallel: 1, verbose: 7},
		},
		{
			// quirk: the message says 1 or more, but 0 and negatives are taken here and only refused later by validateArgs
			name: "VENOM_PARALLEL takes zero and negatives", env: map[string]string{"VENOM_PARALLEL": "-2"}, start: defaultOptions,
			want: options{format: "xml", parallel: -2},
		},
		{
			name:    "environ VENOM_VAR_ entries are appended, the value keeps its = signs",
			environ: []string{"VENOM_VAR_a=1", `VENOM_VAR_d="e=f"`, "VENOM_VAR_=empty", "OTHER=1", "VENOM_FORMAT=json"},
			start:   options{format: "xml", parallel: 1, variables: []string{"z=0"}},
			// VENOM_FORMAT in environ is not read: only os.Getenv is
			want:     options{format: "xml", parallel: 1, variables: []string{"z=0", "a=1", `d="e=f"`, "=empty"}},
			wantVars: []string{"z=0", "a=1", `d="e=f"`, "=empty"},
		},
		{
			// quirk: VENOM_VAR_FROM_FILE also matches the VENOM_VAR_ prefix, so it becomes a variable named FROM_FILE
			name: "VENOM_VAR_FROM_FILE in environ also becomes a variable", env: map[string]string{"VENOM_VAR_FROM_FILE": "a.yml"},
			environ: []string{"VENOM_VAR_FROM_FILE=a.yml"}, start: defaultOptions,
			want:     options{format: "xml", parallel: 1, varFiles: []string{"a.yml"}, variables: []string{"FROM_FILE=a.yml"}},
			wantVars: []string{"FROM_FILE=a.yml"},
		},
		{
			name: "VENOM_VAR and environ entries combine", env: map[string]string{"VENOM_VAR": "a=1"}, environ: []string{"VENOM_VAR_b=2"},
			start: defaultOptions, want: options{format: "xml", parallel: 1, variables: []string{"a=1", "b=2"}}, wantVars: []string{"a=1", "b=2"},
		},
		{
			// quirk: the option is overwritten with ParseBool's false before the error is returned
			name: "an invalid VENOM_STOP_ON_FAILURE fails after resetting the option", env: map[string]string{"VENOM_STOP_ON_FAILURE": "yes"},
			start: options{format: "xml", parallel: 1, stopOnFailure: true}, want: defaultOptions,
			wantErr: "invalid value for VENOM_STOP_ON_FAILURE",
		},
		{
			name: "an invalid VENOM_HTML_REPORT fails after resetting the option", env: map[string]string{"VENOM_HTML_REPORT": "on"},
			start: options{format: "xml", parallel: 1, htmlReport: true}, want: defaultOptions,
			wantErr: "invalid value for VENOM_HTML_REPORT",
		},
		{
			name: "an invalid VENOM_VERBOSE", env: map[string]string{"VENOM_VERBOSE": "x"}, start: defaultOptions, want: defaultOptions,
			wantErr: "invalid value for VENOM_VERBOSE, must be 1, 2 or 3",
		},
		{
			name: "an invalid VENOM_PARALLEL", env: map[string]string{"VENOM_PARALLEL": "1.5"}, start: defaultOptions, want: defaultOptions,
			wantErr: "invalid value for VENOM_PARALLEL, must be a whole number of 1 or more",
		},
		{
			name: "an error stops before the variables read after it",
			env:  map[string]string{"VENOM_FORMAT": "json", "VENOM_STOP_ON_FAILURE": "bad", "VENOM_LIB_DIR": "lib"}, environ: []string{"VENOM_VAR_a=1"},
			start: defaultOptions, want: options{format: "json", parallel: 1}, wantErr: "invalid value for VENOM_STOP_ON_FAILURE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateOptions(t, tt.start)
			for k, val := range tt.env {
				t.Setenv(k, val)
			}
			got, err := initFromEnv(tt.environ)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantVars, got)
			}
			assert.Equal(t, tt.want, currentOptions())
		})
	}
}

// TestOptionParsersPanicOnMalformedEntries pins two panics found while characterising #2031: a variable with no "="
// (mergeVariables slices at index -1) and an environ entry with no "=".
func TestOptionParsersPanicOnMalformedEntries(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "a --var with no = sign",
			run: func() error {
				variablesFlag = ptr([]string{"novalue"})
				initFromCommandArguments(&pflag.Flag{Name: "var", Changed: true})
				return nil
			},
			want: "runtime error: slice bounds out of range [:-1]",
		},
		{
			name: "a variables entry with no = sign in the config file",
			run:  func() error { return initFromReaderConfigFile(strings.NewReader("variables:\n  - 5\n")) },
			want: "runtime error: slice bounds out of range [:-1]",
		},
		{
			name: "an environ VENOM_VAR_ entry with no = sign",
			run: func() error {
				_, err := initFromEnv([]string{"VENOM_VAR_a"})
				return err
			},
			want: "runtime error: index out of range [1] with length 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateOptions(t, defaultOptions)
			var err error
			assert.PanicsWithError(t, tt.want, func() { err = tt.run() })
			assert.NoError(t, err) // PanicsWithError already fails if the call returns; this only checks what it returned
		})
	}
}

// Test_initFromConfigFile_homeFallback pins where initFromConfigFile looks (#2894, RULE-029): a .venomrc in the
// working folder wins, else the one in the home folder is read, else nothing is read and no error is returned.
func Test_initFromConfigFile_homeFallback(t *testing.T) {
	setFormat := func(t *testing.T) {
		t.Helper()
		old := format
		t.Cleanup(func() { format = old })
		format = "xml"
	}
	write := func(t *testing.T, dir, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".venomrc"), []byte(content), 0o600))
	}

	t.Run("home .venomrc is read when the working folder has none", func(t *testing.T) {
		setFormat(t)
		home := t.TempDir()
		write(t, home, "format: json\n")
		t.Setenv("HOME", home)
		t.Chdir(t.TempDir())

		require.NoError(t, initFromConfigFile())
		assert.Equal(t, "json", format)
	})

	t.Run("working folder .venomrc wins over the home one", func(t *testing.T) {
		setFormat(t)
		home, work := t.TempDir(), t.TempDir()
		write(t, home, "format: json\n")
		write(t, work, "format: tap\n")
		t.Setenv("HOME", home)
		t.Chdir(work)

		require.NoError(t, initFromConfigFile())
		assert.Equal(t, "tap", format)
	})

	t.Run("no .venomrc anywhere changes nothing", func(t *testing.T) {
		setFormat(t)
		t.Setenv("HOME", t.TempDir())
		t.Chdir(t.TempDir())

		require.NoError(t, initFromConfigFile())
		assert.Equal(t, "xml", format)
	})

	t.Run("HOME unset: the account's home folder is used and no error comes back", func(t *testing.T) {
		setFormat(t)
		if _, err := user.Current(); err != nil {
			t.Skipf("no account entry for this user: %v", err)
		}
		t.Setenv("HOME", "")
		t.Chdir(t.TempDir())

		require.NoError(t, initFromConfigFile())
	})
}
