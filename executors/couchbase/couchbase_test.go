package couchbase

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ovh/venom"
)

// TestExecutorIsSafeForConcurrentSteps pins the reason Run must not mutate the receiver.
//
// venom registers ONE executor instance per name for the whole process
// (cmd/venom/run/cmd.go: RegisterExecutorBuiltin(name, executorFunc())) and hands that same
// instance to every step of every test file. Since #2020 added --parallel, two test files can run
// steps at the same time, so Run is called concurrently on that one instance.
//
// This executor used to declare Run on a *pointer* receiver — the only one of the seventeen
// builtins that did — and wrote the decoded step straight onto it. Two concurrent couchbase steps
// therefore raced on every field, including dsn/username/password, so one file could connect with
// another file's credentials, and the e.buckets map could be written from two goroutines at once,
// which is an unrecoverable fatal error rather than a test failure.
//
// Run with -race. The connections are expected to fail (there is no server here); what is being
// checked is that the shared instance is not mutated, not what Run returns.
func TestExecutorIsSafeForConcurrentSteps(t *testing.T) {
	venom.InitTestLogger(t) // Run logs through venom's package logger, which is nil until this is called.

	// One instance for every step, exactly as the CLI registers it.
	shared := New()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			step := venom.TestStep{
				"dsn":                      fmt.Sprintf("couchbase://host-%d", i),
				"username":                 fmt.Sprintf("user-%d", i),
				"password":                 fmt.Sprintf("password-%d", i),
				"bucket":                   fmt.Sprintf("bucket-%d", i),
				"wait_until_ready_timeout": 0.01,
			}
			//nolint:errcheck // connecting is expected to fail; this test is a race check
			_, _ = shared.Run(context.Background(), step)
		}(i)
	}
	wg.Wait()

	// The registered instance must be exactly as it was handed over: Run works on a copy, so
	// nothing it decoded may have landed here.
	e, ok := shared.(*Executor)
	if !ok {
		t.Fatalf("New() no longer returns *Executor, got %T", shared)
	}
	if e.DSN != "" || e.Username != "" || e.Password != "" || e.Bucket != "" {
		t.Errorf("Run mutated the shared executor instance: dsn=%q username=%q password=%q bucket=%q"+
			" — every step would then see another step's connection settings",
			e.DSN, e.Username, e.Password, e.Bucket)
	}
	if e.buckets != nil {
		t.Errorf("Run populated the shared instance's bucket cache (%d entries); that map is then"+
			" written from several goroutines at once, which is a fatal error, not a test failure",
			len(e.buckets))
	}
}
