package venom

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHideSensitive(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKey("secrets"), []string{"Joe", "Doe"})
	assert.Equal(t, "__hidden__", HideSensitive(ctx, "Joe"))
	assert.Equal(t, "__hidden__ tests something", HideSensitive(ctx, "Joe tests something"))
	assert.Equal(t, "Dave tests something", HideSensitive(ctx, "Dave tests something"))
	assert.Equal(t, "1234", HideSensitive(ctx, 1234))
	assert.Equal(t, "__hidden__!", HideSensitive(ctx, "Doe!"))
	assert.Equal(t, "__hidden__ __hidden__", HideSensitive(ctx, "Joe Doe"))
}

func TestLogFunctionsRedactSecrets(t *testing.T) {
	InitTestLogger(t)
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)
	logger.Logger.SetLevel(logrus.DebugLevel)
	defer logger.Logger.SetOutput(os.Stdout)

	Info(ctx, "step content: basic_auth_password: %s", "my_secret")
	assert.NotContains(t, buf.String(), "my_secret")
	assert.Contains(t, buf.String(), "__hidden__")

	buf.Reset()
	Debug(ctx, "with vars: %v", H{"basic_auth_password": "my_secret"})
	assert.NotContains(t, buf.String(), "my_secret")
	assert.Contains(t, buf.String(), "__hidden__")

	buf.Reset()
	Error(ctx, "interpolated step:\n%s", "basic_auth_password: my_secret\n")
	assert.NotContains(t, buf.String(), "my_secret")
	assert.Contains(t, buf.String(), "__hidden__")
}

func TestLogFunctionsPreserveNonStringFormatting(t *testing.T) {
	InitTestLogger(t)
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})

	var buf bytes.Buffer
	logger.Logger.SetOutput(&buf)
	logger.Logger.SetLevel(logrus.DebugLevel)
	defer logger.Logger.SetOutput(os.Stdout)

	Info(ctx, "Step #%d-%d content is: %s", 3, 7, "my_secret")
	out := buf.String()
	assert.NotContains(t, out, "%!d")
	assert.Contains(t, out, "Step #3-7 content is: __hidden__")

	buf.Reset()
	Debug(ctx, "count=%d ratio=%.2f ok=%t", 42, 1.5, true)
	out = buf.String()
	assert.NotContains(t, out, "%!")
	assert.Contains(t, out, "count=42 ratio=1.50 ok=true")
}

// TestRedactMapVarsVenomValuesAndKeys proves that the venom.* values holding a secret are hidden while the others keep
// their value and type, and that a key holding a secret is hidden too (#2132 AC 2, R1 and R2). The other values keep
// the rule they had: a declared secret is replaced whole, any other value is cleaned and turned into a string.
func TestRedactMapVarsVenomValuesAndKeys(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})
	vars := H{
		"pwd":                        "my_secret",
		"venom.testcase":             "tc-my_secret",
		"venom.testsuite":            "suite-my_secret",
		"venom.testsuite.totalSteps": 2,
		"venom.libdir":               "lib",
		"tc-my_secret.result.code":   0,
		"other":                      "x",
	}

	redactMapVars(ctx, vars, []string{"pwd"})

	assert.Equal(t, H{
		"pwd":                        "__hidden__",
		"venom.testcase":             "tc-__hidden__",
		"venom.testsuite":            "suite-__hidden__",
		"venom.testsuite.totalSteps": 2,
		"venom.libdir":               "lib",
		"tc-__hidden__.result.code":  "0",
		"other":                      "x",
	}, vars)
}

// TestRedactMapVarsKeyCollision pins the choice when a hidden key meets an existing one: the key that sorts last in the
// original map wins, so the outcome does not depend on map iteration order.
func TestRedactMapVarsKeyCollision(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})
	for range 20 {
		vars := H{"a-__hidden__": "first", "a-my_secret": "second"}
		redactMapVars(ctx, vars, []string{"pwd"})
		assert.Equal(t, H{"a-__hidden__": "second"}, vars)
	}
}

// TestRedactStringMapVenomValuesAndKeys is TestRedactMapVarsVenomValuesAndKeys for the step input variables.
func TestRedactStringMapVenomValuesAndKeys(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})
	vars := map[string]string{
		"pwd":                      "my_secret",
		"venom.testcase":           "tc-my_secret",
		"venom.libdir":             "lib",
		"tc-my_secret.result.code": "0",
	}

	redactStringMap(ctx, vars, []string{"pwd"})

	assert.Equal(t, map[string]string{
		"pwd":                       "__hidden__",
		"venom.testcase":            "tc-__hidden__",
		"venom.libdir":              "lib",
		"tc-__hidden__.result.code": "0",
	}, vars)
}

// TestLogFieldsRedactSecrets proves that the testsuite and testcase fields of a log line are cleaned of the context's
// secrets like its message, and that a field without a secret keeps its value and type (#2132 AC 3).
func TestLogFieldsRedactSecrets(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})
	ctx = context.WithValue(ctx, ContextKey("testsuite"), "suite-my_secret")
	ctx = context.WithValue(ctx, ContextKey("testcase"), "tc-my_secret")
	ctx = context.WithValue(ctx, ContextKey("step"), 3)
	ctx = context.WithValue(ctx, ContextKey("executor"), "exec")

	t.Run("fieldsFromContext", func(t *testing.T) {
		assert.Equal(t, logrus.Fields{
			"testsuite": "suite-__hidden__", "testcase": "tc-__hidden__", "step": 3, "executor": "exec",
		}, fieldsFromContext(ctx, fields...))
	})

	t.Run("a log line", func(t *testing.T) {
		InitTestLogger(t)
		var buf bytes.Buffer
		logger.Logger.SetOutput(&buf)
		defer logger.Logger.SetOutput(os.Stdout)

		Info(ctx, "Starting testcase")
		out := buf.String()
		assert.NotContains(t, out, "my_secret")
		assert.Contains(t, out, "testsuite=suite-__hidden__")
		assert.Contains(t, out, "testcase=tc-__hidden__")
		assert.Contains(t, out, "executor=exec")
	})
}
