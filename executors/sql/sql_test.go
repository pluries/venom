package sql

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const fixtureDSN = "../../tests/sql/sqlite.db"

// withWorkdir sets the test suite's workdir the way the engine does, so a step's file resolves against it.
func withWorkdir(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

func writeSQLFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "query.sql"), []byte(content), 0o600))
	return dir
}

// TestExecutorRun_Commands pins the sqlite "commands" path used by tests/sql.yml: same driver,
// same DSN, same query, same expected row.
func TestExecutorRun_Commands(t *testing.T) {
	venom.InitTestLogger(t)

	step := venom.TestStep{
		"driver":   "sqlite",
		"dsn":      fixtureDSN,
		"commands": []string{"SELECT * FROM test_table;"},
	}

	res, err := Executor{}.Run(context.Background(), step)
	require.NoError(t, err)

	r, ok := res.(Result)
	require.True(t, ok, "Run must return a sql.Result")
	require.Len(t, r.Queries, 1)
	require.Len(t, r.Queries[0].Rows, 1)
	assert.Equal(t, "test row 1", r.Queries[0].Rows[0]["name"])
}

// TestExecutorRun_File pins the file path: the file resolves against the test suite's workdir and
// yields one query result.
func TestExecutorRun_File(t *testing.T) {
	venom.InitTestLogger(t)

	dir := writeSQLFile(t, "SELECT * FROM test_table;")
	step := venom.TestStep{"driver": "sqlite", "dsn": fixtureDSN, "file": "query.sql"}

	res, err := Executor{}.Run(withWorkdir(dir), step)
	require.NoError(t, err)

	r, ok := res.(Result)
	require.True(t, ok, "Run must return a sql.Result")
	require.Len(t, r.Queries, 1)
	require.Len(t, r.Queries[0].Rows, 1)
	assert.Equal(t, "test row 1", r.Queries[0].Rows[0]["name"])
}

// TestExecutorRun_Errors pins each failure's message up to the driver's own text, which a test
// suite author reads in the report: a wrapper added around it would change the prefix.
func TestExecutorRun_Errors(t *testing.T) {
	venom.InitTestLogger(t)

	badFileDir := writeSQLFile(t, "SELECT * FROM no_such_table;")
	tests := []struct {
		name       string
		ctx        context.Context
		step       venom.TestStep
		wantPrefix string
	}{
		{
			name:       "connect failure",
			ctx:        context.Background(),
			step:       venom.TestStep{"driver": "sqlite", "dsn": "/does/not/exist/nope.db", "commands": []string{"SELECT 1;"}},
			wantPrefix: "failed to connect to database: ",
		},
		{
			name:       "failing command names its index",
			ctx:        context.Background(),
			step:       venom.TestStep{"driver": "sqlite", "dsn": fixtureDSN, "commands": []string{"SELECT 1;", "SELECT * FROM no_such_table;"}},
			wantPrefix: "failed to exec command number 1: ",
		},
		{
			name:       "failing file names the resolved path",
			ctx:        withWorkdir(badFileDir),
			step:       venom.TestStep{"driver": "sqlite", "dsn": fixtureDSN, "file": "query.sql"},
			wantPrefix: `failed to exec SQL file "` + filepath.Join(badFileDir, "query.sql") + `": `,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Executor{}.Run(tt.ctx, tt.step)
			require.Error(t, err)
			assert.Truef(t, strings.HasPrefix(err.Error(), tt.wantPrefix), "error %q does not start with %q", err.Error(), tt.wantPrefix)
		})
	}
}

// TestExecutorRun_MissingFile pins that a missing file returns the read error itself, unwrapped.
func TestExecutorRun_MissingFile(t *testing.T) {
	venom.InitTestLogger(t)

	dir := t.TempDir()
	step := venom.TestStep{"driver": "sqlite", "dsn": fixtureDSN, "file": "missing.sql"}

	_, err := Executor{}.Run(withWorkdir(dir), step)
	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr)
	assert.Equal(t, "open "+filepath.Join(dir, "missing.sql")+": no such file or directory", err.Error())
}

// TestExecutorRun_NothingToRun pins that a step with neither commands nor file connects and returns no query.
func TestExecutorRun_NothingToRun(t *testing.T) {
	venom.InitTestLogger(t)

	res, err := Executor{}.Run(context.Background(), venom.TestStep{"driver": "sqlite", "dsn": fixtureDSN})
	require.NoError(t, err)
	assert.Equal(t, Result{Queries: []QueryResult{}}, res)
}

func TestExecutor_ZeroValueResult(t *testing.T) {
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
}

func TestExecutor_GetDefaultAssertions(t *testing.T) {
	got := Executor{}.GetDefaultAssertions()
	require.NotNil(t, got)
	assert.Equal(t, []venom.Assertion{}, got.Assertions)
}
