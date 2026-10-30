package mongo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ovh/venom"
)

// Test fixtures for the characterisation tests of the mongo executor. The client-bound actions run against the mongo
// in-process mock server (mongo_mock_test.go): a real *mongo.Client whose connection answers each command with the
// next scripted reply, with no server, no socket and no docker. The command monitor the mock installs records every
// command the executor sends, so a test pins the filter and options that reach the wire as well as the result.

const (
	testDatabase   = "venom"
	testCollection = "cards"
	testNamespace  = testDatabase + "." + testCollection
)

// testExecutor is the executor as Run leaves it after decoding a step naming testDatabase and testCollection.
var testExecutor = Executor{Database: testDatabase, Collection: testCollection}

// newMock returns the parent of the mock subtests. Each mt.Run gets its own client and an empty reply queue.
func newMock(t *testing.T) *mockT {
	t.Helper()
	venom.InitTestLogger(t)
	return &mockT{T: t}
}

// sentCommands returns the name and the body of every command sent so far, in order.
func sentCommands(mt *mockT) ([]string, []bson.M) {
	mt.Helper()
	var names []string
	var bodies []bson.M
	for _, started := range mt.GetAllStartedEvents() {
		// The driver v2 decodes a nested document into a bson.D unless told otherwise: ask for bson.M, as v1 did.
		decoder := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(started.Command)))
		decoder.DefaultDocumentM()
		var body bson.M
		require.NoError(mt, decoder.Decode(&body))
		names = append(names, started.CommandName)
		bodies = append(bodies, body)
	}
	return names, bodies
}

// sentCommand returns the single command sent so far, failing the test if there is not exactly one.
func sentCommand(mt *mockT) (string, bson.M) {
	mt.Helper()
	names, bodies := sentCommands(mt)
	require.Lenf(mt, names, 1, "commands sent: %v", names)
	return names[0], bodies[0]
}

// sentRawCommand returns the single command sent so far as raw BSON, failing the test if there is not exactly one.
// Unlike the bson.M of sentCommand, raw BSON keeps the key order the executor sent.
func sentRawCommand(mt *mockT) bson.Raw {
	mt.Helper()
	events := mt.GetAllStartedEvents()
	require.Lenf(mt, events, 1, "commands sent: %d", len(events))
	return events[0].Command
}

// orderedKeys returns the keys of the document found at the path of keys in command, in the order they were sent.
// An array element is reached by its index as a key ("0", "1", …).
func orderedKeys(mt *mockT, command bson.Raw, path ...string) []string {
	mt.Helper()
	value, err := command.LookupErr(path...)
	require.NoErrorf(mt, err, "no %v in %s", path, command)
	document, ok := value.DocumentOK()
	require.Truef(mt, ok, "%v is a %s, not a document", path, value.Type)
	elements, err := document.Elements()
	require.NoError(mt, err)
	keys := make([]string, 0, len(elements))
	for _, element := range elements {
		keys = append(keys, element.Key())
	}
	return keys
}

// firstBatch is the reply to a find, aggregate or listCollections whose cursor is exhausted in one batch.
func firstBatch(ns string, docs ...bson.D) bson.D {
	return cursorResponse(0, ns, firstBatchKey, docs...)
}

// serverError is a non-retryable command error reply (code 2, BadValue), so the driver fails on the first attempt.
func serverError() bson.D {
	return commandErrorResponse(commandError{Code: 2, Name: "BadValue", Message: "synthetic failure"})
}

// serverErrorText is how the driver renders serverError.
const serverErrorText = "(BadValue) synthetic failure"

// writeFile writes content to dir/name, creating the parent folders, and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// workdirCtx is the context a step gets inside a suite whose workdir is dir.
func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var."+testsuiteWorkdirVar), dir)
}

// openHandles counts this process's file descriptors open on path. It skips the test where /proc/self/fd does not
// exist, and switches the garbage collector off until the test ends: an *os.File dropped without Close is only closed
// by its finalizer, after a collection, and the count must not change behind the test's back.
func openHandles(t *testing.T, path string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("needs /proc/self/fd: %v", err)
	}
	previous := debug.SetGCPercent(-1)
	t.Cleanup(func() { debug.SetGCPercent(previous) })

	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && target == path {
			count++
		}
	}
	return count
}
