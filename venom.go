package venom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/confluentinc/bincover"
	"github.com/fatih/color"
	"github.com/ovh/venom/assertions"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

var (
	// Version is set with -ldflags "-X github.com/ovh/venom/venom.Version=$(VERSION)"
	Version = "snapshot"
	IsTest  = ""
)

// OSExit is a wrapper for os.Exit
func OSExit(exitCode int) {
	if IsTest != "" {
		bincover.ExitCode = exitCode
	} else {
		os.Exit(exitCode)
	}
}

// ContextKey can be added in context to store contextual infos. Also used by logger.
type ContextKey string

// New instantiates a new venom on venom run cmd
func New() *Venom {
	v := &Venom{
		LogOutput:        os.Stdout,
		PrintFunc:        fmt.Printf,
		executorsBuiltin: map[string]Executor{},
		executorsPlugin:  map[string]Executor{},
		executorsUser:    map[string]Executor{},
		variables:        map[string]interface{}{},
		secrets:          map[string]interface{}{},
		OutputFormat:     "xml",
		Parallel:         1,
	}
	return v
}

type Venom struct {
	LogOutput io.Writer

	PrintFunc        func(format string, a ...interface{}) (n int, err error)
	executorsBuiltin map[string]Executor
	executorsUser    map[string]Executor

	// executorsPluginMu guards executorsPlugin, the only executor registry written while a run is
	// in progress. GetExecutorRunner calls registerPlugin unconditionally for any type that is
	// neither a builtin nor a user executor, so a plugin-typed step re-registers on *every* step,
	// not once, and with test files running concurrently (#2020) those writes would collide
	// (#2019).
	//
	// The re-registration is deliberate, not an oversight — see the call site in GetExecutorRunner.
	// It does not meaningfully contend either: the lock covers only the map write, while
	// plugin.Open and Lookup happen outside it and cost about 1.8 µs whether one file is running
	// or eight (#2023).
	//
	// executorsBuiltin and executorsUser need no lock: both are filled before Process starts — the
	// builtins at startup, the user executors during Parse — and are only read afterwards.
	executorsPluginMu sync.RWMutex
	executorsPlugin   map[string]Executor

	Tests     Tests
	variables H
	secrets   H
	// runSecrets are the names of the variables the run declares as secrets (AddRunSecrets).
	runSecrets []string

	LibDir        string
	OutputFormat  string
	OutputDir     string
	StopOnFailure bool
	// StrictVariables makes a step that names an unknown variable fail, instead of keeping the text as written.
	StrictVariables bool
	HtmlReport      bool //nolint:revive // exported field kept for module compatibility (plugins, library users, upstream merges)
	Verbose         int

	// Parallel is how many test files may run at once. 1 (the default) keeps the sequential path
	// venom has always taken; anything higher runs files through a bounded worker pool (#2020).
	// An embedder that builds a Venom without New() leaves this at 0, which Process reads as 1.
	Parallel int

	// printMu serialises console writes when several test files print at once. It is taken only
	// by a suiteOut that carries a prefix, so the sequential path never touches it (#2021).
	printMu sync.Mutex
}

// suiteOut prints the console output of one test file.
//
// At --parallel 1 it is transparent: no tag, no lock, exactly the bytes venom has always written.
// Above 1 several files print at the same time, so every physical line is prefixed with the file it
// came from and each write is serialised — without that, two goroutines' output tears inside a
// single line and neither file can be read (#2021).
type suiteOut struct {
	v      *Venom
	prefix string // empty when the run is sequential

	// pending holds the part of a line that has been printed but not yet terminated. venom builds
	// a line from several calls — the test case name, then its status — so emitting each call as
	// it arrives let another file's output land in the middle of a line, which is the defect this
	// buffer exists to prevent.
	//
	// It is guarded by v.printMu, the process-wide lock, rather than a per-printer one: the files
	// are what print concurrently, so the emit has to be serialised across printers anyway, and one
	// lock covering both the buffer and the write is simpler than two. Today no single file prints
	// from two goroutines — the only second goroutine is the one a step with a timeout uses
	// (process_teststep.go), and it cannot reach the console: it runs either an executor, none of
	// which holds a *Venom, or RunUserExecutor, whose inner steps are all gated on
	// !fromUserExecutor.
	pending strings.Builder
}

// newSuiteOut returns the printer for one test file. The prefix is padded to the widest filename of
// the run so the tags line up in a column; the file list is fixed before Process starts, so reading
// it here needs no lock.
func (v *Venom) newSuiteOut(ts *TestSuite) *suiteOut {
	if v.Parallel <= 1 {
		return &suiteOut{v: v}
	}
	width := 0
	for i := range v.Tests.TestSuites {
		if n := len(v.Tests.TestSuites[i].Filename); n > width {
			width = n
		}
	}
	return &suiteOut{v: v, prefix: fmt.Sprintf("[%-*s] ", width, ts.Filename)}
}

// out returns the printer of the test file tc belongs to. It falls back to the untagged printer for
// a TestCase built outside any suite — a user executor registered as an assertion function
// (registerUserAssertFunc) has no file to attribute its output to.
func (v *Venom) out(tc *TestCase) *suiteOut {
	if tc != nil && tc.out != nil {
		return tc.out
	}
	return &suiteOut{v: v}
}

func (o *suiteOut) Print(format string, a ...interface{}) {
	o.write(fmt.Sprintf(format, a...))
}

func (o *suiteOut) Println(format string, a ...interface{}) {
	o.write(fmt.Sprintf(format+"\n", a...))
}

func (o *suiteOut) PrintlnTrace(s string) {
	o.PrintlnIndentedTrace(s, "")
}

func (o *suiteOut) PrintlnIndentedTrace(s string, indent string) {
	o.Println("\t  %s%s %s", indent, trace("[trac]"), trace(s))
}

// write sends one already-formatted chunk to the console. Without a prefix it goes straight
// through, which is what keeps the sequential path byte-identical. With one, the chunk joins the
// pending line and only whole lines are emitted.
func (o *suiteOut) write(s string) {
	if o.prefix == "" {
		o.v.PrintFunc("%s", s) //nolint:errcheck // console write; a failed write to the output stream has nothing to recover
		return
	}
	o.v.printMu.Lock()
	defer o.v.printMu.Unlock()
	o.pending.WriteString(s)
	o.emitCompleteLines()
}

// emitCompleteLines writes every line that is now terminated and keeps the remainder pending.
// Callers hold v.printMu.
func (o *suiteOut) emitCompleteLines() {
	buf := o.pending.String()
	end := strings.LastIndex(buf, "\n")
	if end < 0 {
		return
	}
	o.v.PrintFunc("%s", o.tag(buf[:end+1])) //nolint:errcheck // console write; a failed write to the output stream has nothing to recover
	rest := buf[end+1:]
	o.pending.Reset()
	o.pending.WriteString(rest)
}

// Flush emits whatever the file left unterminated, so a line in progress is not swallowed when the
// file ends — an aborted run would otherwise lose the test case it died on.
func (o *suiteOut) Flush() {
	if o.prefix == "" {
		return
	}
	o.v.printMu.Lock()
	defer o.v.printMu.Unlock()
	if o.pending.Len() == 0 {
		return
	}
	o.v.PrintFunc("%s\n", o.tag(o.pending.String())) //nolint:errcheck // console write; a failed write to the output stream has nothing to recover
	o.pending.Reset()
}

// tag prefixes every physical line, not only the first: one call can carry a whole systemout dump,
// and an untagged continuation line is exactly the line a reader cannot attribute.
func (o *suiteOut) tag(s string) string {
	if s == "" {
		return s
	}
	trailing := strings.HasSuffix(s, "\n")
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i := range lines {
		lines[i] = o.prefix + lines[i]
	}
	out := strings.Join(lines, "\n")
	if trailing {
		out += "\n"
	}
	return out
}

var trace = color.New(color.Attribute(90)).SprintFunc()

func (v *Venom) Print(format string, a ...interface{}) {
	v.PrintFunc(format, a...) //nolint:errcheck // console write; a failed write to the output stream has nothing to recover
}

func (v *Venom) Println(format string, a ...interface{}) {
	v.PrintFunc(format+"\n", a...) //nolint:errcheck // console write; a failed write to the output stream has nothing to recover
}

func (v *Venom) PrintlnTrace(s string) {
	v.PrintlnIndentedTrace(s, "")
}

func (v *Venom) PrintlnIndentedTrace(s string, indent string) {
	v.Println("\t  %s%s %s", indent, trace("[trac]"), trace(s))
}

func (v *Venom) AddVariables(variables map[string]interface{}) {
	for k, variable := range variables {
		v.variables[k] = variable
	}
}

func (v *Venom) AddSecrets(secrets map[string]interface{}) {
	for k, s := range secrets {
		v.secrets[k] = s
	}
}

// AddRunSecrets adds the names of variables the run declares as secrets, e.g. in .venomrc. They are merged into every
// suite's own list of secrets when the files are read.
func (v *Venom) AddRunSecrets(names []string) {
	v.runSecrets = append(v.runSecrets, names...)
}

// RegisterExecutorBuiltin register builtin executors
func (v *Venom) RegisterExecutorBuiltin(name string, e Executor) {
	v.executorsBuiltin[name] = e
}

// RegisterExecutorPlugin register plugin executors
func (v *Venom) RegisterExecutorPlugin(name string, e Executor) {
	v.executorsPluginMu.Lock()
	defer v.executorsPluginMu.Unlock()
	v.executorsPlugin[name] = e
}

// lookupExecutorPlugin reads the plugin registry under the read lock. Registration can happen
// while a run is in progress, so every read of executorsPlugin goes through here.
func (v *Venom) lookupExecutorPlugin(name string) (Executor, bool) {
	v.executorsPluginMu.RLock()
	defer v.executorsPluginMu.RUnlock()
	ex, ok := v.executorsPlugin[name]
	return ex, ok
}

// RegisterExecutorUser registers an user executor
func (v *Venom) RegisterExecutorUser(name string, e Executor) error {
	if existing, ok := v.executorsUser[name]; ok {
		filename := "unknown"
		if ux, ok := existing.(UserExecutor); ok {
			filename = ux.Filename
		}
		return fmt.Errorf("executor %q already exists (from file %q)", name, filename)
	}

	v.executorsUser[name] = e
	return nil
}

// GetExecutorRunner initializes a test according to its type
// if no type is provided, exec is default
func (v *Venom) GetExecutorRunner(ctx context.Context, ts TestStep, h H) (context.Context, ExecutorRunner, error) {
	s, err := readRunnerSettings(ts)
	if err != nil {
		return nil, nil, err
	}

	ctx, scope, err := withStepVars(ctx, h)
	if err != nil {
		return ctx, nil, err
	}
	return v.runnerFor(ctx, s, scope)
}

// getExecutorRunnerFlat is GetExecutorRunner for a caller that has already flattened the step's variables: the flattened
// view of scope must be what varsFlattener.flatten gives for the step's scope, and is not modified.
func (v *Venom) getExecutorRunnerFlat(ctx context.Context, ts TestStep, scope *stepScope) (context.Context, ExecutorRunner, error) {
	s, err := readRunnerSettings(ts)
	if err != nil {
		return nil, nil, err
	}
	return v.runnerFor(withStepScope(ctx, scope), s, scope)
}

// runnerFor resolves the runner of a step from its settings: builtin, user, then plugin.
func (v *Venom) runnerFor(ctx context.Context, s runnerSettings, scope *stepScope) (context.Context, ExecutorRunner, error) {
	if s.name == "" {
		return ctx, s.runner(nil, "builtin"), nil
	}

	if ex, ok := v.executorsBuiltin[s.name]; ok {
		return ctx, s.runner(ex, "builtin"), nil
	}

	if ex, ok := v.executorsUser[s.name]; ok {
		return ctx, s.runner(ex, "user"), nil
	}

	// A plugin-typed step re-registers its executor on every step: there is no "already loaded?"
	// check here, and adding one was considered and rejected in #2023.
	//
	// It is cheaper than it looks — plugin.Open caches by path, so the .so is read once and a
	// repeat call costs about 1.8 µs, measured, with no contention between concurrent files. And
	// skipping it would change behaviour: registerPlugin resolves <workdir>/lib/<name>.so before
	// the global lib/<name>.so, so two test files in different directories can each ship their own
	// plugin under the same name. Caching by name would give the second file the first one's
	// plugin. Verified with two hello.so variants; see docs/feat/2023/validation.md.
	workdir, _ := scope.Get("venom.testsuite.workdir")
	loadErr := v.registerPlugin(ctx, s.name, workdir)
	if loadErr != nil {
		Debug(ctx, "executor %q is not implemented as plugin - err:%v", s.name, loadErr)
	}

	if ex, ok := v.lookupExecutorPlugin(s.name); ok {
		return ctx, s.runner(ex, "plugin"), nil
	}
	err := fmt.Errorf("user executor %q not found - loaded executors are: %v", s.name, reflect.ValueOf(v.executorsUser).MapKeys())
	if loadErr != nil && pluginFileExists(workdir, s.name) {
		// RULE-020's message stays as it is when there is no plugin file; a file that exists but cannot be loaded (built
		// with another Go patch or another venom) says why.
		err = fmt.Errorf("%w (loading it as a plugin failed: %w)", err, loadErr)
	}
	return ctx, nil, err
}

// runnerSettings are the step keys that configure its runner, whatever the executor.
type runnerSettings struct {
	name    string
	retry   int
	retryIf []string
	delay   int
	timeout int
	info    []string
}

// readRunnerSettings reads the executor name and the runner keys of a step.
func readRunnerSettings(ts TestStep) (runnerSettings, error) {
	var s runnerSettings
	var err error
	if s.name, err = stepExecutorName(ts); err != nil {
		return s, err
	}
	if s.retry, err = ts.IntValue("retry"); err != nil {
		return s, err
	}
	if s.retryIf, err = ts.StringSliceValue("retry_if"); err != nil {
		return s, err
	}
	if s.delay, err = ts.IntValue("delay"); err != nil {
		return s, err
	}
	if s.timeout, err = ts.IntValue("timeout"); err != nil {
		return s, err
	}
	s.info, _ = ts.StringSliceValue("info") //nolint:errcheck // info only feeds report display (newExecutorRunner), no control-flow impact if malformed
	return s, nil
}

// stepExecutorName returns the step's type, or exec for an untyped step with a script or a command.
func stepExecutorName(ts TestStep) (string, error) {
	name, err := ts.StringValue("type")
	if err != nil {
		return "", err
	}
	// script and command only matter when inferring exec: a typed step may be a user executor
	// whose inputs reuse these keys with list or map values.
	if name == "" {
		script, err := ts.StringValue("script")
		if err != nil {
			return "", err
		}
		command, err := ts.StringSliceValue("command")
		if err != nil {
			return "", err
		}
		if script != "" || len(command) != 0 {
			name = "exec"
		}
	}
	return name, nil
}

// runner builds the step's runner around ex, reported as an executor of kind stype.
func (s runnerSettings) runner(ex Executor, stype string) ExecutorRunner {
	return newExecutorRunner(ex, s.name, stype, s.retry, s.retryIf, s.delay, s.timeout, s.info)
}

// withStepVars exposes each of h's variables in ctx as "var.<name>", and their names as "vars".
func withStepVars(ctx context.Context, h H) (context.Context, *stepScope, error) {
	vars, err := dumpHPreserveCase(h)
	if err != nil {
		return ctx, nil, err
	}
	scope := newWholeScope(vars, vars)
	return withStepScope(ctx, scope), scope, nil
}

// withStepScope exposes the flattened variables of scope in ctx. The scope is not copied and must not change while the
// context is in use.
func withStepScope(ctx context.Context, scope *stepScope) context.Context {
	return &stepVarsContext{Context: ctx, scope: scope}
}

// stepVarsContext answers ContextKey("var.<name>") and ContextKey("vars") from a step's scope, and everything else from
// its parent. A context.WithValue per variable gave the same answers, but a lookup of any key set before them, a log
// field such as the test case name, walked a chain as long as the scope, 27 000 links for a large suite (#2465).
type stepVarsContext struct {
	context.Context
	scope *stepScope

	keysOnce sync.Once
	keys     []string
}

func (c *stepVarsContext) Value(key interface{}) interface{} {
	if k, ok := key.(ContextKey); ok {
		if k == "vars" {
			c.keysOnce.Do(func() { c.keys = c.scope.Names() })
			return c.keys
		}
		if name, ok := strings.CutPrefix(string(k), "var."); ok {
			if val, ok := c.scope.Get(name); ok {
				return val
			}
		}
	}
	return c.Context.Value(key)
}

func (v *Venom) getUserExecutorFilesPath(ctx context.Context, vars map[string]string) []string {
	var libpaths []string
	// ensure libpaths is unique
	seen := make(map[string]struct{})

	if v.LibDir != "" {
		for _, lp := range strings.Split(v.LibDir, string(os.PathListSeparator)) {
			abs := strings.TrimSpace(lp)
			if abs == "" {
				continue
			}
			absPath, err := filepath.Abs(abs)
			if err == nil {
				seen[absPath] = struct{}{}
				libpaths = append(libpaths, absPath)
			}
		}
	}

	relLib := path.Join(vars["venom.testsuite.workdir"], "lib")
	if absRelLib, err := filepath.Abs(relLib); err == nil {
		if _, exists := seen[absRelLib]; !exists {
			libpaths = append(libpaths, absRelLib)
			seen[absRelLib] = struct{}{}
		}
	}
	// use a map to avoid duplicates
	filepaths := make(map[string]bool)

	for _, p := range libpaths {
		p = strings.TrimSpace(p)

		err := filepath.Walk(p, func(fp string, _ os.FileInfo, _ error) error {
			switch ext := filepath.Ext(fp); ext {
			case ".yml", ".yaml":
				filepaths[fp] = true
			}
			return nil
		})
		if err != nil {
			Warn(ctx, "Unable to list files in lib directory %q: %v", p, err)
		}
	}

	userExecutorFiles := make([]string, len(filepaths))
	i := 0
	for p := range filepaths {
		userExecutorFiles[i] = p
		i++
	}

	sort.Strings(userExecutorFiles)
	if len(userExecutorFiles) == 0 {
		Warn(ctx, "no user executor yml file selected")
	}
	return userExecutorFiles
}

func (v *Venom) registerUserExecutors(ctx context.Context) error {
	vars, err := DumpStringPreserveCase(v.variables)
	if err != nil {
		return errors.Wrapf(err, "unable to parse variables")
	}

	executorsPath := v.getUserExecutorFilesPath(ctx, vars)

	for _, f := range executorsPath {
		Debug(ctx, "Reading %v", f)
		content, err := os.ReadFile(f)
		if err != nil {
			return errors.Wrapf(err, "unable to read file %q", f)
		}

		ex := readPartialYML(content, "executor")
		if len(ex) == 0 {
			return errors.Errorf("missing key 'executor' in %q", f)
		}

		name := strings.Replace(ex, "executor:", "", 1)
		name = strings.TrimSpace(name)

		inputs := readPartialYML(content, "input")

		ux := UserExecutor{
			Filename:  f,
			Executor:  name,
			RawInputs: []byte(inputs),
			Raw:       content,
		}

		err = v.RegisterExecutorUser(ux.Executor, ux)
		if err != nil {
			return errors.Wrapf(err, "unable to register user executor %q from file %q", ux.Executor, f)
		}
		Info(ctx, "User executor %q registered", ux.Executor)

		if strings.HasPrefix(ux.Executor, "Should") {
			err = v.registerUserAssertFunc(ctx, ux.Executor)
			if err != nil {
				return errors.Wrapf(err, "unable to register user executor %q from file %q as user assertion", ux.Executor, f)
			}
		}
	}

	return nil
}

func (v *Venom) registerUserAssertFunc(ctx context.Context, name string) error {
	_, e, err := v.GetExecutorRunner(ctx, TestStep{"type": name}, H{})
	if err != nil {
		return errors.Wrapf(err, "unable to load user executor %q", name)
	}

	err = assertions.RegisterUserAssertFunc(name, func(actual interface{}, expected ...interface{}) error {
		// Prepare a minimal context in which the user executor can be run
		// We only need to register the operands in the var set
		// as parent contexts will already have been templated
		tc := &TestCase{TestCaseInput: TestCaseInput{Name: name}}
		tc.TestSuiteVars.Add("a", actual)
		if len(expected) > 0 {
			tc.TestSuiteVars.Add("b", expected[0])
		}
		tc.TestSuiteVars.Add("argv", expected)
		ts := &TestStepResult{}

		_, err := v.RunUserExecutor(ctx, e, tc, ts, TestStep{})

		// We reformat any sub-assertions errors to avoid redundant texts
		if len(ts.Errors) > 0 {
			msg := make([]string, len(ts.Errors))
			for _, e := range ts.Errors {
				msg = append(msg, fmt.Sprintf(`  %d: Sub-assertion %q failed. %s`,
					e.StepNumber,
					RemoveNotPrintableChar(e.Assertion),
					RemoveNotPrintableChar(e.Error.Error()),
				))
			}
			return errors.New(strings.Join(msg, "\n"))
		} else if err != nil {
			return errors.Wrapf(err, "user assertion failed during execution")
		}

		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "unable to register user assertion function %q", name)
	}

	return nil
}

// validPluginName refuses an executor name that could reach a file outside the lib folders (#2299).
func validPluginName(name string) error {
	if name == "" || name == "." || strings.ContainsAny(name, "/\\\x00") || strings.Contains(name, "..") {
		return fmt.Errorf("unable to load plugin %q: the name must not contain a path", name)
	}
	return nil
}

// pluginCandidates are the two places registerPlugin looks for <name>.so: the test suite's lib folder, then lib/ of the
// working directory.
func pluginCandidates(workdir, name string) (suitePath, binPath string) {
	return path.Join(workdir, "lib", name+".so"), path.Join("lib", name+".so")
}

// pluginFileExists tells whether a plugin file is there to be loaded, in either place registerPlugin looks.
func pluginFileExists(workdir, name string) bool {
	suitePath, binPath := pluginCandidates(workdir, name)
	return statOK(suitePath) || statOK(binPath)
}

// statOK tells whether p can be stat-ed: a missing file, or any other error, is false (unlike fileExists, which
// dereferences the result of a failed stat other than not-exist).
func statOK(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (v *Venom) registerPlugin(_ context.Context, name, workdir string) error {
	if err := validPluginName(name); err != nil {
		return err
	}
	// try to load from testsuite path
	suitePath, binPath := pluginCandidates(workdir, name)
	p, err := plugin.Open(suitePath)
	if err != nil {
		suiteErr := err
		// try to load from venom binary path
		p, err = plugin.Open(binPath)
		if err != nil {
			// say why the file that is there could not be loaded, not why the one that is not there could not
			if statOK(suitePath) {
				err = suiteErr
			}
			return fmt.Errorf("unable to load plugin %q.so: %w", name, err)
		}
	}

	symbolExecutor, err := p.Lookup("Plugin")
	if err != nil {
		return err
	}

	executor, ok := symbolExecutor.(Executor)
	if !ok {
		return fmt.Errorf("plugin %q: Plugin symbol does not implement the Executor interface", name)
	}
	v.RegisterExecutorPlugin(name, executor)

	return nil
}

func VarFromCtx(ctx context.Context, varname string) interface{} {
	i := ctx.Value(ContextKey("var." + varname))
	return i
}

func StringVarFromCtx(ctx context.Context, varname string) string {
	i := ctx.Value(ContextKey("var." + varname))
	return cast.ToString(i)
}

func StringSliceVarFromCtx(ctx context.Context, varname string) []string {
	i := ctx.Value(ContextKey("var." + varname))
	return cast.ToStringSlice(i)
}

func IntVarFromCtx(ctx context.Context, varname string) int {
	i := ctx.Value(ContextKey("var." + varname))
	return cast.ToInt(i)
}

func BoolVarFromCtx(ctx context.Context, varname string) bool {
	i := ctx.Value(ContextKey("var." + varname))
	return cast.ToBool(i)
}

func StringMapInterfaceVarFromCtx(ctx context.Context, varname string) map[string]interface{} {
	i := ctx.Value(ContextKey("var." + varname))
	return cast.ToStringMap(i)
}

func StringMapStringVarFromCtx(ctx context.Context, varname string) map[string]string {
	i := ctx.Value(ContextKey("var." + varname))
	return cast.ToStringMapString(i)
}

func AllVarsFromCtx(ctx context.Context) H {
	i := ctx.Value(ContextKey("vars"))
	allKeys := cast.ToStringSlice(i)
	res := H{}
	for _, k := range allKeys {
		res.Add(k, VarFromCtx(ctx, k))
	}
	return res
}

func JSONUnmarshal(btes []byte, i interface{}) error {
	d := json.NewDecoder(bytes.NewReader(btes))
	d.UseNumber()
	return d.Decode(i)
}
