package dbfixtures

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"slices"

	fixtures "github.com/go-testfixtures/testfixtures/v3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	migrate "github.com/rubenv/sql-migrate"
	"modernc.org/sqlite"

	// SQL drivers.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/ovh/venom"
)

// Name of the executor.
const Name = "dbfixtures"

// The pure Go SQLite driver registers itself as "sqlite". Suites have always said "sqlite3", the name the cgo driver
// used (#1976: that driver cannot work in a CGO_ENABLED=0 build), so the same driver answers to it as well.
func init() {
	if !slices.Contains(sql.Drivers(), "sqlite3") {
		sql.Register("sqlite3", &sqlite.Driver{})
	}
}

// dialectName maps the database name a suite gives to the name sql-migrate and testfixtures know.
func dialectName(database string) string {
	if database == "sqlite" {
		return "sqlite3"
	}
	return database
}

// New returns a new executor that can load
// database fixtures.
func New() venom.Executor {
	return &Executor{}
}

// Executor is a venom executor that can load
// fixtures in many databases, using YAML schemas.
type Executor struct {
	Files              []string `json:"files" yaml:"files"`
	Folder             string   `json:"folder" yaml:"folder"`
	Database           string   `json:"database" yaml:"database"`
	DSN                string   `json:"dsn" yaml:"dsn"`
	Schemas            []string `json:"schemas" yaml:"schemas"`
	Migrations         string   `json:"migrations" yaml:"migrations"`
	MigrationsTable    string   `json:"migrationsTable" yaml:"migrationsTable"`
	SkipResetSequences bool     `json:"skipResetSequences" yaml:"skipResetSequences"`
}

// Result represents a step result.
type Result struct {
	Executor Executor `json:"executor,omitempty" yaml:"executor,omitempty"`
}

// Run implements the venom.Executor interface for Executor.
func (e Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// Transform step to Executor instance.
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	// Connect to the database and ping it.
	venom.Debug(ctx, "connecting to database %s, %s\n", e.Database, e.DSN)

	db, err := sql.Open(e.Database, e.DSN)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to database")
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		return nil, errors.Wrapf(err, "failed to ping database")
	}

	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")

	if err = e.prepare(ctx, db, workdir); err != nil {
		return nil, err
	}

	// Load fixtures in the databases.
	if err = loadFixtures(ctx, db, e.Files, e.Folder, getDialect(dialectName(e.Database), e.SkipResetSequences), workdir); err != nil {
		return nil, err
	}
	r := Result{Executor: e}

	return r, nil
}

// prepare loads the schemas, or else runs the migrations, when the step names them.
func (e Executor) prepare(ctx context.Context, db *sql.DB, workdir string) error {
	if len(e.Schemas) != 0 {
		return e.loadSchemas(ctx, db, workdir)
	}
	if e.Migrations == "" {
		return nil
	}
	venom.Debug(ctx, "loading migrations from folder %s\n", e.Migrations)
	if e.MigrationsTable != "" {
		migrate.SetTable(e.MigrationsTable)
	}
	migrations := &migrate.FileMigrationSource{Dir: path.Join(workdir, e.Migrations)}
	n, err := migrate.Exec(db, dialectName(e.Database), migrations, migrate.Up)
	if err != nil {
		return fmt.Errorf("failed to apply up migrations: %s", err)
	}
	venom.Debug(ctx, "applied %d migrations\n", n)
	return nil
}

// loadSchemas executes each schema file, resolved against the test suite's workdir.
func (e Executor) loadSchemas(ctx context.Context, db *sql.DB, workdir string) error {
	for _, s := range e.Schemas {
		venom.Debug(ctx, "loading schema from file %s\n", s)
		s = path.Join(workdir, s)
		sbytes, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		if _, err = db.Exec(string(sbytes)); err != nil {
			return errors.Wrapf(err, "failed to exec schema from file %q", s)
		}
	}
	return nil
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// The engine only takes default assertions in the pointer form and ignores any other silently: refuse it at build time.
var _ interface{ GetDefaultAssertions() *venom.StepAssertions } = Executor{}

// GetDefaultAssertions return the default assertions of the executor.
func (e Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{}}
}

// loadFixtures loads the fixtures in the database.
// It gives priority to the fixtures files found in folder,
// and switch to the list of files if no folder was specified.
func loadFixtures(ctx context.Context, db *sql.DB, files []string, folder string, dialect func(*fixtures.Loader) error, workdir string) error {
	if folder != "" {
		venom.Debug(ctx, "loading fixtures from folder %s\n", path.Join(workdir, folder))
		loader, err := fixtures.New(
			// By default the package refuse to load if the database
			// does not contains "test" to avoid wiping a production db.
			fixtures.DangerousSkipTestDatabaseCheck(),
			fixtures.Database(db),
			fixtures.Directory(path.Join(workdir, folder)),
			dialect)
		if err != nil {
			return errors.Wrapf(err, "failed to create folder loader")
		}
		if err = loader.Load(); err != nil {
			return errors.Wrapf(err, "failed to load fixtures from folder %q", path.Join(workdir, folder))
		}
		return nil
	}
	if len(files) != 0 {
		venom.Debug(ctx, "loading fixtures from files: %v\n", files)
		for i := range files {
			files[i] = path.Join(workdir, files[i])
		}
		loader, err := fixtures.New(
			// By default the package refuse to load if the database
			// does not contains "test" to avoid wiping a production db.
			fixtures.DangerousSkipTestDatabaseCheck(),
			fixtures.Database(db),
			fixtures.Files(files...),
			dialect)
		if err != nil {
			return errors.Wrapf(err, "failed to create files loader")
		}
		if err = loader.Load(); err != nil {
			return errors.Wrapf(err, "failed to load fixtures from files")
		}
		return nil
	}
	venom.Debug(ctx, "neither files or folder parameter was used\n")

	return nil
}

func getDialect(name string, skipResetSequences bool) func(*fixtures.Loader) error {
	switch name {
	case "postgres":
		return func(l *fixtures.Loader) error {
			if err := fixtures.Dialect("postgresql")(l); err != nil {
				return err
			}
			if skipResetSequences {
				if err := fixtures.SkipResetSequences()(l); err != nil {
					return err
				}
			}
			return nil
		}
	case "mysql":
		return fixtures.Dialect("mysql")
	case "sqlite3":
		return fixtures.Dialect("sqlite3")
	}
	return nil
}
