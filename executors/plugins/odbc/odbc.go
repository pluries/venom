package main

import (
	"C"
	"context"

	"github.com/mitchellh/mapstructure"

	// ODBC
	_ "github.com/alexbrainman/odbc"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/sqlquery"
)

// Name of the executor.
const Name = "odbc"

// Plugin var is mandatory, it's used by venom to register the executor
var Plugin = Executor{}

// The engine only takes default assertions in the pointer form and ignores any other silently: refuse it at build time.
var _ interface{ GetDefaultAssertions() *venom.StepAssertions } = Executor{}

// Executor is a venom executor can execute SQL queries
type Executor struct {
	File     string   `json:"file,omitempty" yaml:"file,omitempty"`
	Commands []string `json:"commands,omitempty" yaml:"commands,omitempty"`
	DSN      string   `json:"dsn" yaml:"dsn"`
}

// Run implements the venom.Executor interface for Executor. It runs the same query engine as the
// sql executor (executors/internal/sqlquery), with the driver fixed to "odbc".
func (e Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// Transform step to Executor instance.
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	return sqlquery.Run(ctx, "odbc", e.DSN, e.Commands, e.File)
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return sqlquery.Result{}
}

// GetDefaultAssertions return the default assertions of the executor.
func (e Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{}}
}
