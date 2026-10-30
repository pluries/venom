// Package sqlquery runs SQL commands or a SQL file over a database/sql driver and returns the rows,
// for the executors that talk to SQL databases (the sql executor and the odbc plugin). It registers
// no driver: each executor imports the drivers it offers.
package sqlquery

import (
	"context"
	"os"
	"path"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/ovh/venom"
)

// Rows represents an array of Row
type Rows []Row

// Row represents a row return by a SQL query.
type Row map[string]interface{}

// QueryResult represents a rows return by a SQL query execution.
type QueryResult struct {
	Rows Rows `json:"rows,omitempty" yaml:"rows,omitempty"`
}

// Result represents a step result.
type Result struct {
	Queries []QueryResult `json:"queries,omitempty" yaml:"queries,omitempty"`
}

// Run connects to the database over driver/dsn and executes commands, or the SQL file at file when
// commands is empty: one QueryResult per command, or a single one for the file.
func Run(ctx context.Context, driver, dsn string, commands []string, file string) (Result, error) {
	// Connect to the database and ping it.
	venom.Debug(ctx, "connecting to database %s, %s\n", driver, dsn)
	db, err := sqlx.Connect(driver, dsn)
	if err != nil {
		return Result{}, errors.Wrapf(err, "failed to connect to database")
	}
	defer db.Close()

	results := []QueryResult{}
	// The file is only a fallback when no command is given.
	switch {
	case len(commands) != 0:
		results, err = runCommands(ctx, db, commands)
	case file != "":
		results, err = runFile(ctx, db, file)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Queries: results}, nil
}

// runCommands executes each command in order and returns one QueryResult per command.
func runCommands(ctx context.Context, db *sqlx.DB, commands []string) ([]QueryResult, error) {
	results := []QueryResult{}
	for i, s := range commands {
		venom.Debug(ctx, "Executing command number %d\n", i)
		rows, err := db.Queryx(s)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to exec command number %d", i)
		}
		r, err := handleRows(rows)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse SQL rows for command number %d", i)
		}
		results = append(results, QueryResult{Rows: r})
	}
	return results, nil
}

// runFile executes the SQL file, resolved against the test suite's workdir, as a single query.
func runFile(ctx context.Context, db *sqlx.DB, file string) ([]QueryResult, error) {
	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")
	f := path.Join(workdir, file)
	venom.Debug(ctx, "loading SQL file from %s\n", f)
	sbytes, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	rows, err := db.Queryx(string(sbytes))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to exec SQL file %q", f)
	}
	r, err := handleRows(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse SQL rows for SQL file %q", f)
	}
	return []QueryResult{{Rows: r}}, nil
}

// handleRows iter on each SQL rows result sets and serialize it into a []Row.
func handleRows(rows *sqlx.Rows) ([]Row, error) {
	defer rows.Close()
	res := []Row{}
	for rows.Next() {
		row := make(Row)
		if err := rows.MapScan(row); err != nil {
			return nil, err
		}
		res = append(res, row)
	}
	if err := rows.Err(); err != nil {
		return res, err
	}
	return res, nil
}
