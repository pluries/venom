package sqlquery

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// The package registers no driver; its tests bring the one they use.
	_ "modernc.org/sqlite"

	"github.com/ovh/venom"
)

const fixtureDSN = "../../../tests/sql/sqlite.db"

func withWorkdir(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

func writeSQLFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "query.sql"), []byte(content), 0o600))
	return dir
}

func TestRun_Commands(t *testing.T) {
	venom.InitTestLogger(t)

	r, err := Run(context.Background(), "sqlite", fixtureDSN, []string{"SELECT * FROM test_table;", "SELECT 1 AS one;"}, "")
	require.NoError(t, err)
	require.Len(t, r.Queries, 2, "one result per command")
	require.Len(t, r.Queries[0].Rows, 1)
	assert.Equal(t, "test row 1", r.Queries[0].Rows[0]["name"])
	assert.Equal(t, int64(1), r.Queries[1].Rows[0]["one"])
}

func TestRun_CommandsWinOverFile(t *testing.T) {
	venom.InitTestLogger(t)

	r, err := Run(withWorkdir(t.TempDir()), "sqlite", fixtureDSN, []string{"SELECT 1 AS one;"}, "missing.sql")
	require.NoError(t, err, "the file is only read when no command is given")
	require.Len(t, r.Queries, 1)
}

func TestRun_File(t *testing.T) {
	venom.InitTestLogger(t)

	dir := writeSQLFile(t, "SELECT * FROM test_table;")
	r, err := Run(withWorkdir(dir), "sqlite", fixtureDSN, nil, "query.sql")
	require.NoError(t, err)
	require.Len(t, r.Queries, 1)
	assert.Equal(t, "test row 1", r.Queries[0].Rows[0]["name"])
}

func TestRun_NothingToRun(t *testing.T) {
	venom.InitTestLogger(t)

	r, err := Run(context.Background(), "sqlite", fixtureDSN, nil, "")
	require.NoError(t, err)
	assert.Equal(t, Result{Queries: []QueryResult{}}, r)
}

func TestRun_Errors(t *testing.T) {
	venom.InitTestLogger(t)

	badFileDir := writeSQLFile(t, "SELECT * FROM no_such_table;")
	tests := []struct {
		name       string
		ctx        context.Context
		driver     string
		dsn        string
		commands   []string
		file       string
		wantPrefix string
	}{
		{"unknown driver", context.Background(), "no-such-driver", fixtureDSN, []string{"SELECT 1;"}, "", `failed to connect to database: sql: unknown driver "no-such-driver"`},
		{"connect failure", context.Background(), "sqlite", "/does/not/exist/nope.db", []string{"SELECT 1;"}, "", "failed to connect to database: "},
		{"failing command", context.Background(), "sqlite", fixtureDSN, []string{"SELECT 1;", "SELECT * FROM no_such_table;"}, "", "failed to exec command number 1: "},
		{"failing file", withWorkdir(badFileDir), "sqlite", fixtureDSN, nil, "query.sql", `failed to exec SQL file "` + filepath.Join(badFileDir, "query.sql") + `": `},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Run(tt.ctx, tt.driver, tt.dsn, tt.commands, tt.file)
			require.Error(t, err)
			assert.Equal(t, Result{}, r)
			assert.Truef(t, strings.HasPrefix(err.Error(), tt.wantPrefix), "error %q does not start with %q", err.Error(), tt.wantPrefix)
		})
	}
}

func TestRun_MissingFile(t *testing.T) {
	venom.InitTestLogger(t)

	dir := t.TempDir()
	_, err := Run(withWorkdir(dir), "sqlite", fixtureDSN, nil, "missing.sql")
	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr)
	assert.Equal(t, "open "+filepath.Join(dir, "missing.sql")+": no such file or directory", err.Error())
}
