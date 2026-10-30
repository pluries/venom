package mongo

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ovh/venom"
)

// Characterisation tests for the mongo executor (#2031, reopen item C2). This file pins the parts that need no
// server: fixture-file listing, the documents read from a file or from inline strings, the dispatch of an action that
// never reaches the client, and Run's own connect path. mongo_actions_test.go drives the client-bound actions against
// the driver's mock deployment. They pin what the executor does today, odd parts included; nothing here is a fix.

func TestFixtureFiles(t *testing.T) {
	t.Run("only .yaml and .yml files, in directory order, subfolders skipped", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"pokemons.yaml", "cards.yml", "notes.txt", "README", "decks.json", "trainers.YML"} {
			writeFile(t, dir, name, "- name: synthetic\n")
		}
		// A folder is skipped even when its name looks like a fixture, and nothing below it is read.
		writeFile(t, dir, filepath.Join("folder.yml", "hidden.yml"), "- name: synthetic\n")
		writeFile(t, dir, filepath.Join("nested", "items.yml"), "- name: synthetic\n")

		got, err := fixtureFiles(dir)
		require.NoError(t, err)
		// os.ReadDir sorts by name; the extension match is case-sensitive, so trainers.YML is not a fixture.
		assert.Equal(t, []string{"cards.yml", "pokemons.yaml"}, got)
	})

	t.Run("an empty folder gives an empty, non-nil list", func(t *testing.T) {
		got, err := fixtureFiles(t.TempDir())
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("a missing folder is os.ReadDir's error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		got, err := fixtureFiles(missing)
		require.ErrorIs(t, err, os.ErrNotExist)
		assert.EqualError(t, err, "open "+missing+": no such file or directory")
		assert.Nil(t, got)
	})
}

func TestDocumentsFromFile(t *testing.T) {
	oid, err := bson.ObjectIDFromHex("5d505646cf6d4fe581014ab2")
	require.NoError(t, err)

	tests := []struct {
		name    string
		content string
		want    []any
	}{
		{
			name:    "a stream of several JSON objects, in file order (the README's jsonlist format)",
			content: "{\n  \"suit\": \"hearts\",\n  \"value\": \"queen\"\n}\n{\n  \"suit\": \"diamonds\",\n  \"value\": \"three\"\n}\n",
			want: []any{
				bson.M{"suit": "hearts", "value": "queen"},
				bson.M{"suit": "diamonds", "value": "three"},
			},
		},
		{
			name:    "objects need no separator between them",
			content: `{"value":"ace"}{"value":"two"}`,
			want:    []any{bson.M{"value": "ace"}, bson.M{"value": "two"}},
		},
		{
			name:    "extended JSON is decoded: $oid becomes an ObjectID, nested objects and arrays keep bson types",
			content: `{"_id":{"$oid":"5d505646cf6d4fe581014ab2"},"stats":{"hp":35},"types":["electric"]}`,
			want: []any{bson.M{
				"_id":   oid,
				"stats": bson.M{"hp": int32(35)},
				"types": bson.A{"electric"},
			}},
		},
		{
			// The file goes through encoding/json into float64 before bson sees it: a whole number loses precision past
			// 2^53 and "1.0" turns into an int32, where the same text inline stays exact (see TestDocumentsFromStrings).
			name:    "numbers go through float64: large integers are rounded, 1.0 becomes an int32",
			content: `{"big":9007199254740993,"one":1.0,"half":1.5,"small":7}`,
			want: []any{bson.M{
				"big":   int64(9007199254740992),
				"one":   int32(1),
				"half":  1.5,
				"small": int32(7),
			}},
		},
		{
			name:    "a file with only whitespace gives no documents and no error",
			content: "\n  \n",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, filepath.Join("fixtures", "cards.jsonlist"), tt.content)

			got, err := documentsFromFile(workdirCtx(dir), filepath.Join("fixtures", "cards.jsonlist"))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("an empty path gives nil, nil without touching the disk", func(t *testing.T) {
		got, err := documentsFromFile(workdirCtx(filepath.Join(t.TempDir(), "missing")), "")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("an absolute path is still joined under the workdir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, filepath.Join("abs", "cards.json"), `{"value":"king"}`)

		got, err := documentsFromFile(workdirCtx(dir), "/abs/cards.json")
		require.NoError(t, err)
		assert.Equal(t, []any{bson.M{"value": "king"}}, got)
	})

	t.Run("without a workdir in the context the path is relative to the working directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "cards.json", `{"value":"jack"}`)
		t.Chdir(dir)

		got, err := documentsFromFile(context.Background(), "cards.json")
		require.NoError(t, err)
		assert.Equal(t, []any{bson.M{"value": "jack"}}, got)
	})

	errorTests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "a truncated stream is encoding/json's error", content: `{"value":"ace"}{"value":`, want: "unexpected EOF"},
		{name: "a malformed object is encoding/json's syntax error", content: `{"value" "ace"}`,
			want: `invalid character '"' after object key`},
		{name: "an invalid extended-JSON value is bson's error", content: `{"_id":{"$oid":"not-an-object-id"}}`,
			want: "error decoding key _id: the provided hex string is not a valid ObjectID"},
		{name: "a JSON array is not a stream of documents", content: `[{"value":"ace"},{"value":"two"}]`,
			want: "cannot decode array into a bson.M"},
		{name: "a bare scalar is not a document", content: `42`, want: "cannot decode 32-bit integer into a bson.M"},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "cards.jsonlist", tt.content)

			got, err := documentsFromFile(workdirCtx(dir), "cards.jsonlist")
			assert.EqualError(t, err, tt.want)
			assert.Nil(t, got, "no partial result on error")
		})
	}

	t.Run("the file is left open (D113)", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "cards.json", `{"value":"ace"}`)
		require.Equal(t, 0, openHandles(t, path))

		for range 3 {
			_, err := documentsFromFile(workdirCtx(dir), "cards.json")
			require.NoError(t, err)
		}
		assert.Equal(t, 3, openHandles(t, path), "one descriptor per call, none closed")
	})

	t.Run("a missing file is os.Open's error, with the joined path", func(t *testing.T) {
		dir := t.TempDir()
		got, err := documentsFromFile(workdirCtx(dir), "missing.json")
		require.ErrorIs(t, err, os.ErrNotExist)
		assert.EqualError(t, err, "open "+filepath.Join(dir, "missing.json")+": no such file or directory")
		assert.Nil(t, got)
	})
}

func TestDocumentsFromStrings(t *testing.T) {
	t.Run("nil and empty give nil, nil", func(t *testing.T) {
		for _, in := range [][]string{nil, {}} {
			got, err := documentsFromStrings(in)
			require.NoError(t, err)
			assert.Nil(t, got)
		}
	})

	t.Run("each string is one document, in order, numbers exact", func(t *testing.T) {
		got, err := documentsFromStrings([]string{
			`{"suit":"hearts","value":"queen"}`,
			`{"big":9007199254740993,"one":1.0,"_id":{"$oid":"5d505646cf6d4fe581014ab2"}}`,
		})
		require.NoError(t, err)
		oid, err := bson.ObjectIDFromHex("5d505646cf6d4fe581014ab2")
		require.NoError(t, err)
		assert.Equal(t, []any{
			bson.M{"suit": "hearts", "value": "queen"},
			bson.M{"big": int64(9007199254740993), "one": 1.0, "_id": oid},
		}, got)
	})

	t.Run("one bad string fails the whole list, with no partial result", func(t *testing.T) {
		got, err := documentsFromStrings([]string{`{"value":"ace"}`, `{"value":`})
		assert.EqualError(t, err, "invalid JSON input; unexpected end of input at position 0")
		assert.Nil(t, got)
	})

	t.Run("an empty string is an error", func(t *testing.T) {
		got, err := documentsFromStrings([]string{""})
		assert.EqualError(t, err, "positioned on invalid, but attempted to read embedded document")
		assert.Nil(t, got)
	})
}

func TestNew(t *testing.T) {
	assert.Equal(t, &Executor{}, New())
}

func TestRunAction_withoutClient(t *testing.T) {
	// An unknown or missing type is not an error: the action is skipped and its entry in Result.Actions stays nil.
	for _, action := range []map[string]any{{"type": "unknown"}, {}, {"type": "Count"}} {
		got, err := testExecutor.runAction(context.Background(), nil, action)
		require.NoError(t, err, "action %v", action)
		assert.Nil(t, got, "action %v", action)
	}
}

// redactSynthetic is the synthetic password of TestRedactURI and redactMasked what redactURI puts in its place. The
// URIs and the expected values are built by concatenation so that no source line holds a literal URI with credentials,
// which the repository's secret scan and gosec G101 refuse.
const (
	redactSynthetic = "s3cr3t-synthetic"
	redactMasked    = "xxxxx"
)

// TestRedactURI proves #2037 AC 3: the URI Run logs at debug level never carries the password, whatever its shape,
// including the shapes url.Parse rejects while the driver accepts them; the host list stays readable.
func TestRedactURI(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		want   string
		hidden string // the part of the user info that must not survive
		host   string // the part that must stay
	}{
		{
			name: "single host", uri: "mongodb://user:" + redactSynthetic + "@localhost:27017",
			want: "mongodb://user:" + redactMasked + "@localhost:27017", hidden: redactSynthetic, host: "localhost:27017",
		},
		{
			name: "several hosts with options", uri: "mongodb://user:" + redactSynthetic + "@h1:27017,h2:27017/db?replicaSet=rs",
			want: "mongodb://user:" + redactMasked + "@h1:27017,h2:27017/db?replicaSet=rs", hidden: redactSynthetic, host: "h1:27017,h2:27017",
		},
		{
			name: "srv", uri: "mongodb+srv://user:" + redactSynthetic + "@cluster.example.test/db",
			want: "mongodb+srv://user:" + redactMasked + "@cluster.example.test/db", hidden: redactSynthetic, host: "cluster.example.test",
		},
		{
			name: "escaped password", uri: "mongodb://user:" + "p%40ss" + "@localhost:27017",
			want: "mongodb://user:" + redactMasked + "@localhost:27017", hidden: "p%40ss", host: "localhost:27017",
		},
		{
			name: "user without password is unchanged", uri: "mongodb://user@localhost:27017",
			want: "mongodb://user@localhost:27017", host: "localhost:27017",
		},
		{
			name: "no user info is unchanged", uri: "mongodb://localhost:27017/?replicaSet=rs",
			want: "mongodb://localhost:27017/?replicaSet=rs", host: "localhost:27017",
		},
		{name: "empty", uri: "", want: ""},
		// url.Parse reads this one as scheme "user" with an opaque part, so Redacted would keep the password.
		{
			name: "no // after the scheme", uri: "user:" + redactSynthetic + "@localhost:27017",
			want: redactMasked + "@localhost:27017", hidden: redactSynthetic, host: "localhost:27017",
		},
		// url.Parse cuts the string at "#" and then "?" before it looks for the "@" of the user info: these parse without
		// an error, with no user, and Redacted would leave the password. The driver accepts all of them.
		{
			name: "a # in the password", uri: "mongodb://admin:" + "2024#" + redactSynthetic + "@db:27017/?replicaSet=rs",
			want: "mongodb://" + redactMasked + "@db:27017/?replicaSet=rs", hidden: redactSynthetic, host: "db:27017",
		},
		{
			name: "a ? in the password", uri: "mongodb://admin:" + "12?" + redactSynthetic + "@db:27017/db",
			want: "mongodb://" + redactMasked + "@db:27017/db", hidden: redactSynthetic, host: "db:27017",
		},
		{
			name: "a ? in the user", uri: "mongodb://ad?min:" + redactSynthetic + "@db:27017",
			want: "mongodb://" + redactMasked + "@db:27017", hidden: redactSynthetic, host: "db:27017",
		},
		// The price of the rule above: an "@" in an option of a URI without user info is cut as if it ended the user info.
		// Hiding too much is accepted, showing a password is not.
		{
			name: "an at sign in an option without user info is over-hidden", uri: "mongodb://localhost:27017/?appName=a" + "@" + "b",
			want: "mongodb://" + redactMasked + "@b", host: "b",
		},
		// url.Parse rejects the three shapes below, and its error quotes the URI: the fallback cuts the user info by hand.
		{
			name: "several IPv6 hosts", uri: "mongodb://user:" + redactSynthetic + "@[::1]:27017,[::2]:27017",
			want: "mongodb://" + redactMasked + "@[::1]:27017,[::2]:27017", hidden: redactSynthetic, host: "[::1]:27017,[::2]:27017",
		},
		{
			name: "a space in the user", uri: "mongodb://us er:" + redactSynthetic + "@localhost",
			want: "mongodb://" + redactMasked + "@localhost", hidden: redactSynthetic, host: "localhost",
		},
		{
			name: "an invalid escape in the password", uri: "mongodb://user:" + "s3cr%zz" + "@localhost",
			want: "mongodb://" + redactMasked + "@localhost", hidden: "s3cr", host: "localhost",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactURI(tt.uri)
			assert.Equal(t, tt.want, got)
			if tt.hidden != "" {
				assert.NotContains(t, got, tt.hidden)
			}
			assert.Contains(t, got, tt.host)
		})
	}
}

// unreachableURI points at a unix socket that does not exist, so a server selection fails deterministically after
// 200 ms instead of the driver's 30 s default.
func unreachableURI(t *testing.T) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "missing.sock")
	return "mongodb://" + url.QueryEscape(socket) + "/?serverSelectionTimeoutMS=200"
}

func TestRun(t *testing.T) {
	venom.InitTestLogger(t)

	t.Run("a step that does not decode fails before connecting", func(t *testing.T) {
		got, err := Executor{}.Run(context.Background(), venom.TestStep{"uri": 42})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "'URI' expected type 'string', got unconvertible type 'int'")
		assert.Nil(t, got)
	})

	t.Run("an invalid URI is a connect error", func(t *testing.T) {
		got, err := Executor{}.Run(context.Background(), venom.TestStep{"uri": "localhost:27017"})
		assert.EqualError(t, err, `failed to connect to database: error parsing uri: scheme must be "mongodb" or "mongodb+srv"`)
		assert.Nil(t, got)
	})

	t.Run("the connection is lazy: a step with no action succeeds against an unreachable server", func(t *testing.T) {
		got, err := Executor{}.Run(context.Background(), venom.TestStep{"uri": unreachableURI(t), "database": testDatabase})
		require.NoError(t, err)
		assert.Equal(t, Result{Actions: []map[string]any{}}, got)
	})

	t.Run("actions that never reach the server leave a nil entry each", func(t *testing.T) {
		step := venom.TestStep{
			"uri":     unreachableURI(t),
			"actions": []map[string]any{{"type": "unknown"}, {"type": "other"}},
		}
		got, err := Executor{}.Run(context.Background(), step)
		require.NoError(t, err)
		assert.Equal(t, Result{Actions: []map[string]any{nil, nil}}, got)
	})

	t.Run("the first failing action stops the run and drops the results before it", func(t *testing.T) {
		step := venom.TestStep{
			"uri":     unreachableURI(t),
			"actions": []map[string]any{{"type": "unknown"}, {"type": "loadFixtures"}, {"type": "count"}},
		}
		got, err := Executor{}.Run(context.Background(), step)
		assert.EqualError(t, err, "folder is required")
		assert.Nil(t, got)
	})

	t.Run("an action that needs the server fails once server selection times out", func(t *testing.T) {
		step := venom.TestStep{
			"uri":        unreachableURI(t),
			"database":   testDatabase,
			"collection": testCollection,
			"actions":    []map[string]any{{"type": "count"}},
		}
		got, err := Executor{}.Run(context.Background(), step)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server selection error: context deadline exceeded")
		assert.Contains(t, err.Error(), "missing.sock")
		assert.Nil(t, got)
	})
}
