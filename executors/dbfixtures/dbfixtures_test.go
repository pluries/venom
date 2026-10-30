package dbfixtures

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// testsDir holds the schemas, migrations and fixtures the tests/db_fixtures.yml suite uses.
const testsDir = "../../tests"

// withWorkdir sets the test suite's workdir the way the engine does, so a step's files resolve against it.
func withWorkdir(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

// TestExecutorRun_sqliteNames pins the SQLite database names (#2896, #1976): "sqlite3" is the name suites have always
// used and must keep working; "sqlite" is the pure Go driver's own name and is accepted too. Both load a schema or
// run migrations, then the fixtures, which goes through the sql-migrate and testfixtures dialects.
func TestExecutorRun_sqliteNames(t *testing.T) {
	venom.InitTestLogger(t)
	for _, database := range []string{"sqlite3", "sqlite"} {
		t.Run(database+" schema and fixtures", func(t *testing.T) {
			step := venom.TestStep{
				"database": database,
				"dsn":      ":memory:",
				"schemas":  []string{"dbfixtures/testdata/schemas/sqlite3.sql"},
				"folder":   "dbfixtures/testdata/fixtures",
			}
			_, err := Executor{}.Run(withWorkdir(testsDir), step)
			require.NoError(t, err)
		})

		t.Run(database+" migrations and fixtures", func(t *testing.T) {
			step := venom.TestStep{
				"database":   database,
				"dsn":        ":memory:",
				"migrations": "dbfixtures/testdata/migrations",
				"folder":     "dbfixtures/testdata/fixtures",
			}
			_, err := Executor{}.Run(withWorkdir(testsDir), step)
			require.NoError(t, err)
		})
	}
}

// TestExecutorRun_unknownDatabase pins the error for a name no driver knows.
func TestExecutorRun_unknownDatabase(t *testing.T) {
	venom.InitTestLogger(t)
	step := venom.TestStep{"database": "nosuchdb", "dsn": "x"}
	_, err := Executor{}.Run(withWorkdir(testsDir), step)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to database")
}

// TestDriversRegistered pins that both SQLite names are registered with database/sql.
func TestDriversRegistered(t *testing.T) {
	assert.Contains(t, sql.Drivers(), "sqlite3")
	assert.Contains(t, sql.Drivers(), "sqlite")
}
