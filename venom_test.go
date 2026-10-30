package venom

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runnerVenom is the Venom the GetExecutorRunner characterisation tests of #2031 resolve against: builtins "exec" and
// "builtin", a user executor "user" registered the way Parse registers one, and "both" registered as a builtin and as
// a user executor.
func runnerVenom(t *testing.T) *Venom {
	t.Helper()
	InitTestLogger(t)
	v := New()
	v.RegisterExecutorBuiltin("exec", sleeperExecutor{})
	v.RegisterExecutorBuiltin("builtin", sleeperExecutor{})
	v.RegisterExecutorBuiltin("both", sleeperExecutor{})
	require.NoError(t, v.RegisterExecutorUser("user", UserExecutor{Executor: "user"}))
	require.NoError(t, v.RegisterExecutorUser("both", UserExecutor{Executor: "both"}))
	return v
}

// TestGetExecutorRunnerResolvesTheExecutor characterises GetExecutorRunner before #2031 splits it: the name it infers
// when the step has no type, and which registry answers — builtin before user.
func TestGetExecutorRunnerResolvesTheExecutor(t *testing.T) {
	tests := []struct {
		name         string
		step         TestStep
		wantName     string
		wantType     string
		wantExecutor Executor
	}{
		{name: "no type, no script, no command: empty name and no executor", step: TestStep{}, wantName: "", wantType: "builtin"},
		{name: "a script infers exec", step: TestStep{"script": "echo hi"}, wantName: "exec", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{name: "a script of spaces still infers exec", step: TestStep{"script": " "}, wantName: "exec", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{name: "a command list infers exec", step: TestStep{"command": []interface{}{"ls", "-l"}}, wantName: "exec", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{name: "a command string infers exec", step: TestStep{"command": "ls"}, wantName: "exec", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		// quirk: a number is cast to a string, so it counts as a command
		{name: "a command number infers exec", step: TestStep{"command": 42}, wantName: "exec", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{name: "an empty command list does not infer exec", step: TestStep{"command": []interface{}{}}, wantName: "", wantType: "builtin"},
		{name: "an empty command string does not infer exec", step: TestStep{"command": ""}, wantName: "", wantType: "builtin"},
		{name: "an empty script does not infer exec", step: TestStep{"script": ""}, wantName: "", wantType: "builtin"},
		{name: "a builtin type", step: TestStep{"type": "builtin"}, wantName: "builtin", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{name: "a user executor type", step: TestStep{"type": "user"}, wantName: "user", wantType: "user", wantExecutor: UserExecutor{Executor: "user"}},
		{name: "a builtin wins over a user executor of the same name", step: TestStep{"type": "both"}, wantName: "both", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{name: "an explicit exec type", step: TestStep{"type": "exec", "script": "echo"}, wantName: "exec", wantType: "builtin", wantExecutor: sleeperExecutor{}},
		{
			name:     "with a type, script and command are not read, so their shape does not matter",
			step:     TestStep{"type": "user", "script": []interface{}{"x"}, "command": map[string]interface{}{"a": 1}},
			wantName: "user", wantType: "user", wantExecutor: UserExecutor{Executor: "user"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := runnerVenom(t)
			ctx, runner, err := v.GetExecutorRunner(context.Background(), tt.step, H{})
			require.NoError(t, err)
			require.NotNil(t, ctx)
			require.NotNil(t, runner)
			assert.Equal(t, tt.wantName, runner.Name())
			assert.Equal(t, tt.wantType, runner.Type())
			assert.Equal(t, tt.wantExecutor, runner.GetExecutor())
		})
	}
}

// TestGetExecutorRunnerCopiesStepSettings pins the retry, retry_if, delay, timeout and info values the returned runner
// exposes, including how spf13/cast converts them.
func TestGetExecutorRunnerCopiesStepSettings(t *testing.T) {
	tests := []struct {
		name        string
		step        TestStep
		wantRetry   int
		wantRetryIf []string
		wantDelay   int
		wantTimeout int
		wantInfo    []string
	}{
		{name: "nothing set", step: TestStep{"type": "builtin"}, wantRetryIf: []string{}, wantInfo: []string{}},
		{
			name:      "every setting",
			step:      TestStep{"type": "builtin", "retry": 3, "retry_if": []interface{}{"a", "b"}, "delay": 2, "timeout": 10, "info": []interface{}{"i1", "i2"}},
			wantRetry: 3, wantRetryIf: []string{"a", "b"}, wantDelay: 2, wantTimeout: 10, wantInfo: []string{"i1", "i2"},
		},
		{name: "numbers written as strings", step: TestStep{"type": "builtin", "retry": "3", "delay": "4", "timeout": "5"}, wantRetry: 3, wantDelay: 4, wantTimeout: 5, wantRetryIf: []string{}, wantInfo: []string{}},
		{name: "a decimal is truncated", step: TestStep{"type": "builtin", "retry": 2.7}, wantRetry: 2, wantRetryIf: []string{}, wantInfo: []string{}},
		{name: "nil counts as zero", step: TestStep{"type": "builtin", "retry": nil}, wantRetryIf: []string{}, wantInfo: []string{}},
		{name: "a single retry_if and info string become one-item lists", step: TestStep{"type": "builtin", "retry_if": "result.code ShouldEqual 1", "info": "hello"}, wantRetryIf: []string{"result.code ShouldEqual 1"}, wantInfo: []string{"hello"}},
		// quirk: a malformed info is dropped silently (the error is discarded), where every other setting fails
		{name: "a malformed info becomes nil", step: TestStep{"type": "builtin", "info": map[string]interface{}{"a": 1}}, wantRetryIf: []string{}, wantInfo: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := runnerVenom(t)
			_, runner, err := v.GetExecutorRunner(context.Background(), tt.step, H{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantRetry, runner.Retry())
			assert.Equal(t, tt.wantRetryIf, runner.RetryIf())
			assert.Equal(t, tt.wantDelay, runner.Delay())
			assert.Equal(t, tt.wantTimeout, runner.Timeout())
			assert.Equal(t, tt.wantInfo, runner.Info())
		})
	}
}

// TestGetExecutorRunnerRejectsMalformedAttributes pins the exact error, and the nil context and runner, for each
// attribute of the wrong shape. The attributes are checked in the order of the table.
func TestGetExecutorRunnerRejectsMalformedAttributes(t *testing.T) {
	tests := []struct {
		name    string
		step    TestStep
		wantErr string
	}{
		{name: "type is a list", step: TestStep{"type": []interface{}{"exec"}}, wantErr: `attribute "type" is not an string`},
		{name: "type is a map", step: TestStep{"type": map[string]interface{}{}}, wantErr: `attribute "type" is not an string`},
		{name: "script is a list", step: TestStep{"script": []interface{}{"echo"}}, wantErr: `attribute "script" is not an string`},
		{name: "script is a map", step: TestStep{"script": map[string]interface{}{}}, wantErr: `attribute "script" is not an string`},
		{name: "command is a map", step: TestStep{"command": map[string]interface{}{"a": 1}}, wantErr: `attribute "command" is neither a string nor a string array`},
		{name: "retry is a word", step: TestStep{"type": "builtin", "retry": "x"}, wantErr: `attribute "retry" is not an integer`},
		{name: "retry_if is a map", step: TestStep{"type": "builtin", "retry_if": map[string]interface{}{}}, wantErr: `attribute "retry_if" is neither a string nor a string array`},
		{name: "delay is a list", step: TestStep{"type": "builtin", "delay": []interface{}{}}, wantErr: `attribute "delay" is not an integer`},
		{name: "timeout is a word", step: TestStep{"type": "builtin", "timeout": "t"}, wantErr: `attribute "timeout" is not an integer`},
		{name: "the first malformed attribute is the one reported", step: TestStep{"type": "builtin", "timeout": "t", "retry": "x"}, wantErr: `attribute "retry" is not an integer`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := runnerVenom(t)
			ctx, runner, err := v.GetExecutorRunner(context.Background(), tt.step, H{})
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, ctx)
			assert.Nil(t, runner)
		})
	}
}

// TestGetExecutorRunnerUnknownType pins the plugin fallback: a type that is neither builtin nor user is looked up as a
// plugin, and without a lib/<type>.so the error names only the user executors. Unlike the attribute errors, the
// context comes back set.
func TestGetExecutorRunnerUnknownType(t *testing.T) {
	tests := []struct {
		name     string
		step     TestStep
		register func(v *Venom)
		wantErr  string
	}{
		{name: "no user executor", step: TestStep{"type": "nope"}, register: func(*Venom) {}, wantErr: `user executor "nope" not found - loaded executors are: []`},
		{
			name: "the error lists the user executors, not the builtins", step: TestStep{"type": "nope"},
			register: func(v *Venom) {
				v.RegisterExecutorBuiltin("builtin", sleeperExecutor{})
				require.NoError(t, v.RegisterExecutorUser("user", UserExecutor{Executor: "user"}))
			},
			wantErr: `user executor "nope" not found - loaded executors are: [user]`,
		},
		// quirk: a number is cast to a string and looked up as a name
		{name: "a numeric type", step: TestStep{"type": 1}, register: func(*Venom) {}, wantErr: `user executor "1" not found - loaded executors are: []`},
		{name: "exec inferred but not registered", step: TestStep{"script": "echo"}, register: func(*Venom) {}, wantErr: `user executor "exec" not found - loaded executors are: []`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			v := New()
			tt.register(v)
			ctx, runner, err := v.GetExecutorRunner(context.Background(), tt.step, H{"a": "1"})
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, runner)
			require.NotNil(t, ctx)
			assert.Equal(t, "1", ctx.Value(ContextKey("var.a")))
		})
	}

	t.Run("an executor already in the plugin registry is found even without its .so", func(t *testing.T) {
		v := runnerVenom(t)
		v.RegisterExecutorPlugin("plug", sleeperExecutor{})
		_, runner, err := v.GetExecutorRunner(context.Background(), TestStep{"type": "plug", "retry": 1}, H{})
		require.NoError(t, err)
		assert.Equal(t, "plug", runner.Name())
		assert.Equal(t, "plugin", runner.Type())
		assert.Equal(t, 1, runner.Retry())
		assert.Equal(t, sleeperExecutor{}, runner.GetExecutor())
	})
}

// TestGetExecutorRunnerSetsContextVariables pins the context values GetExecutorRunner adds: one ContextKey("var.<k>")
// per key of the flattened variables (DumpStringPreserveCase, with its __Len__ / __Type__ extras), and
// ContextKey("vars") listing those keys.
func TestGetExecutorRunnerSetsContextVariables(t *testing.T) {
	v := runnerVenom(t)
	h := H{"a": "1", "Mixed": "case", "m": map[string]interface{}{"k": "v"}, "l": []interface{}{1}}
	ctx, _, err := v.GetExecutorRunner(context.Background(), TestStep{"type": "builtin"}, h)
	require.NoError(t, err)

	keys, ok := ctx.Value(ContextKey("vars")).([]string)
	require.True(t, ok, "ContextKey(\"vars\") holds a []string")
	sort.Strings(keys)
	assert.Equal(t, []string{
		"Mixed", "__Len__", "__Type__", "a", "l", "l.__Len__", "l.__Type__", "l.l0", "m", "m.__Len__", "m.__Type__", "m.k",
	}, keys)

	got := map[string]interface{}{}
	for _, k := range keys {
		got[k] = ctx.Value(ContextKey("var." + k))
	}
	assert.Equal(t, map[string]interface{}{
		"Mixed": "case", "__Len__": "4", "__Type__": "Map", "a": "1",
		"l": "[1]", "l.__Len__": "1", "l.__Type__": "Array", "l.l0": "1",
		"m": `{"k":"v"}`, "m.__Len__": "1", "m.__Type__": "Map", "m.k": "v",
	}, got)
}

// stepContextVars is the variable scope of the step context tests (#1980): a nested value #2017's cache keeps, a
// shadowed pair it keeps out, and an upper-case name, the shapes any future change to withStepVars must reproduce (#2405).
func stepContextVars() H {
	return H{
		"scalar": "s",
		// Nested and shared by no other name: the kind of value a cache serves from its previous flatten.
		"resp": nestedBody(2, 2),
		// "body" and "body.status" share a prefix (a captured map and a pre-flattened suite value). Chosen not to produce
		// the same flat key, which would leave the winner to go-dump's map order.
		"body":        map[string]interface{}{"code": "200"},
		"body.status": "ok",
		// The step's context keeps the case of the name: "var.Upper", never "var.upper".
		"Upper": "U",
	}
}

// TestGetExecutorRunnerContextMatchesTheFlatten pins the context GetExecutorRunner returns against the flatten it is
// built from, for a scope with nested and shadowed names: every key of DumpStringPreserveCase(h) is readable as
// ContextKey("var.<key>") with its value, ContextKey("vars") lists exactly those keys, and AllVarsFromCtx, the way an
// executor reads them all back, gives the same map.
func TestGetExecutorRunnerContextMatchesTheFlatten(t *testing.T) {
	v := runnerVenom(t)
	h := stepContextVars()
	want, err := DumpStringPreserveCase(h)
	require.NoError(t, err)

	ctx, runner, err := v.GetExecutorRunner(context.Background(), TestStep{"type": "builtin"}, h)
	require.NoError(t, err)
	require.NotNil(t, runner)

	wantVars := H{}
	for k, val := range want {
		assert.Equal(t, val, ctx.Value(ContextKey("var."+k)), "var.%s", k)
		wantVars[k] = val
	}
	keys, ok := ctx.Value(ContextKey("vars")).([]string)
	require.True(t, ok, "ContextKey(\"vars\") holds a []string")
	assert.ElementsMatch(t, slices.Collect(maps.Keys(want)), keys)
	assert.Equal(t, wantVars, AllVarsFromCtx(ctx))

	// A few values by name, so the test does not only compare the context with its own source.
	assert.Equal(t, "item-1", StringVarFromCtx(ctx, "resp.resp0.child.child1.name"))
	assert.Equal(t, "200", StringVarFromCtx(ctx, "body.code"))
	assert.Equal(t, "ok", StringVarFromCtx(ctx, "body.status"))
	assert.Equal(t, "U", StringVarFromCtx(ctx, "Upper"))
	assert.Nil(t, VarFromCtx(ctx, "upper"), "the context keeps the case of the name")
	assert.Equal(t, "5", StringVarFromCtx(ctx, "__Len__"), "the count of top-level names")
}

// TestStepVarsContextAnswersAsOneValuePerVariable pins withStepScope (#2465) to the chain of context.WithValue it
// replaced: the same variables, the same list of names, a variable shadowing the parent's, and the parent's other keys.
func TestStepVarsContextAnswersAsOneValuePerVariable(t *testing.T) {
	type otherKey struct{}
	parent := context.WithValue(context.Background(), ContextKey("testcase"), "tc")
	parent = context.WithValue(parent, ContextKey("var.kept"), "from parent")
	parent = context.WithValue(parent, ContextKey("var.shadowed"), "from parent")
	parent = context.WithValue(parent, otherKey{}, 7)
	vars := map[string]string{"a": "1", "b.c": "2", "shadowed": "from vars", "": "empty name"}

	want := parent
	var names []string
	for k, val := range vars {
		want = context.WithValue(want, ContextKey("var."+k), val)
		names = append(names, k)
	}
	want = context.WithValue(want, ContextKey("vars"), names)

	got := withStepScope(parent, newWholeScope(vars, vars))
	for _, key := range []interface{}{
		ContextKey("var.a"), ContextKey("var.b.c"), ContextKey("var.shadowed"), ContextKey("var."), ContextKey("var.kept"),
		ContextKey("var.absent"), ContextKey("testcase"), ContextKey("var"), otherKey{}, "var.a",
	} {
		require.Equal(t, want.Value(key), got.Value(key), "%v", key)
	}
	require.ElementsMatch(t, want.Value(ContextKey("vars")), got.Value(ContextKey("vars")))

	empty := withStepScope(context.Background(), newWholeScope(map[string]string{}, map[string]string{}))
	require.Equal(t, []string{}, empty.Value(ContextKey("vars")))
}

// TestRegisterPluginErrorCarriesTheOpenReason: a lib/<name>.so that plugin.Open refuses is reported with Open's reason,
// not only its name (#2919).
func TestRegisterPluginErrorCarriesTheOpenReason(t *testing.T) {
	InitTestLogger(t)
	workdir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workdir, "lib"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "lib", "x.so"), []byte("not a shared object"), 0o600))

	err := New().registerPlugin(context.Background(), "x", workdir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unable to load plugin "x".so: `)
	assert.Contains(t, err.Error(), "plugin.Open(")
	// the reason belongs to the file that exists (the suite's own lib/), not to the fallback lib/x.so of the working
	// directory, which is not there: its error would read "realpath failed"
	assert.Contains(t, err.Error(), workdir)
	assert.NotContains(t, err.Error(), "realpath failed")
}

// TestGetExecutorRunnerSaysWhyAPluginDidNotLoad pins what a step's error says when its type is neither builtin nor user
// (#2849, RULE-020): a missing plugin file keeps the message of RULE-020 byte for byte; a plugin file that exists but
// cannot be loaded (built on another Go patch or another venom) adds the reason from plugin.Open.
func TestGetExecutorRunnerSaysWhyAPluginDidNotLoad(t *testing.T) {
	InitTestLogger(t)
	const old = `user executor "x" not found - loaded executors are: []`

	t.Run("no plugin file: the message of RULE-020, unchanged", func(t *testing.T) {
		workdir := t.TempDir()
		t.Chdir(t.TempDir())
		_, _, err := New().GetExecutorRunner(context.Background(), TestStep{"type": "x"}, H{"venom.testsuite.workdir": workdir})
		require.Error(t, err)
		assert.Equal(t, old, err.Error())
	})

	t.Run("a plugin file that cannot be loaded: the reason is added", func(t *testing.T) {
		workdir := t.TempDir()
		t.Chdir(t.TempDir())
		require.NoError(t, os.MkdirAll(filepath.Join(workdir, "lib"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(workdir, "lib", "x.so"), []byte("not a shared object"), 0o600))
		_, _, err := New().GetExecutorRunner(context.Background(), TestStep{"type": "x"}, H{"venom.testsuite.workdir": workdir})
		require.Error(t, err)
		assert.Contains(t, err.Error(), old)
		assert.Contains(t, err.Error(), `loading it as a plugin failed: unable to load plugin "x".so: `)
		assert.Contains(t, err.Error(), "plugin.Open(")
	})
}
