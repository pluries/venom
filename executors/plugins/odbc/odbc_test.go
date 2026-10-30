package main

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// A DSN in a directory that does not exist: the ODBC driver manager rejects it, and a wrongly used
// sqlite driver could not create it either, so no test leaves a file behind.
const bogusDSN = "/nonexistent-venom-2101/not-a-dsn"

// SQLDriverConnect is the ODBC call the odbc driver names in its connect errors; the text after it
// comes from the machine's unixODBC configuration and is not pinned.
const odbcConnectPrefix = "failed to connect to database: SQLDriverConnect: "

// TestExecutorRun_DriverFixedToODBC pins that the plugin connects through the "odbc" driver: the
// error comes from a real SQLDriverConnect call (this machine has the unixODBC headers, CLAUDE.md
// § Build), not from an unknown or different driver.
func TestExecutorRun_DriverFixedToODBC(t *testing.T) {
	venom.InitTestLogger(t)

	step := venom.TestStep{"dsn": bogusDSN, "commands": []string{"SELECT 1;"}}

	_, err := Executor{}.Run(context.Background(), step)
	require.Error(t, err)
	assert.Truef(t, strings.HasPrefix(err.Error(), odbcConnectPrefix), "error %q does not start with %q", err.Error(), odbcConnectPrefix)
}

// TestExecutorRun_IgnoresDriverField pins that a stray "driver" key in an odbc step is dropped: the
// plugin's Executor has no Driver field, so the connect still goes through ODBC.
func TestExecutorRun_IgnoresDriverField(t *testing.T) {
	venom.InitTestLogger(t)

	step := venom.TestStep{"driver": "sqlite", "dsn": bogusDSN, "commands": []string{"SELECT 1;"}}

	_, err := Executor{}.Run(context.Background(), step)
	require.Error(t, err)
	assert.Truef(t, strings.HasPrefix(err.Error(), odbcConnectPrefix), "error %q does not start with %q", err.Error(), odbcConnectPrefix)
}

// TestExecutor_ZeroValueResult pins what the engine records from the zero value (appendResultVars
// dumps it), captured from the plugin's code before #2101 — not the Go type, which changed.
func TestExecutor_ZeroValueResult(t *testing.T) {
	got, err := venom.DumpString(Executor{}.ZeroValueResult())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"__type__":                "Result",
		"result.__Len__":          "1",
		"result.queries":          "null",
		"result.queries.__Len__":  "0",
		"result.queries.__Type__": "Array",
	}, got)
}

func TestExecutor_GetDefaultAssertions(t *testing.T) {
	got := Executor{}.GetDefaultAssertions()
	require.NotNil(t, got)
	assert.Equal(t, []venom.Assertion{}, got.Assertions)
}
