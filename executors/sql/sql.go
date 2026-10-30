package sql

import (
	"context"

	"github.com/mitchellh/mapstructure"

	// MySQL drivers
	_ "github.com/go-sql-driver/mysql"

	// Postgres driver
	_ "github.com/lib/pq"

	// Oracle
	_ "github.com/sijms/go-ora"

	// Sqlite
	_ "modernc.org/sqlite"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/sqlquery"
)

// Name of the executor.
const Name = "sql"

// New returns a new executor that can execute SQL queries
func New() venom.Executor {
	return &Executor{}
}

// Executor is a venom executor can execute SQL queries
type Executor struct {
	File     string   `json:"file,omitempty" yaml:"file,omitempty"`
	Commands []string `json:"commands,omitempty" yaml:"commands,omitempty"`
	Driver   string   `json:"driver" yaml:"driver"`
	DSN      string   `json:"dsn" yaml:"dsn"`
}

// Rows represents an array of Row
type Rows = sqlquery.Rows

// Row represents a row return by a SQL query.
type Row = sqlquery.Row

// QueryResult represents a rows return by a SQL query execution.
type QueryResult = sqlquery.QueryResult

// Result represents a step result.
type Result = sqlquery.Result

// Run implements the venom.Executor interface for Executor.
func (e Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// Transform step to Executor instance.
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	return sqlquery.Run(ctx, e.Driver, e.DSN, e.Commands, e.File)
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
