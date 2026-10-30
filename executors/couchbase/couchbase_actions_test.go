package couchbase

import (
	"context"
	"testing"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// unreachableDSN points at a loopback port nobody listens on: gocb connects lazily, so
// building a cluster succeeds and only a real operation would fail.
const unreachableDSN = "couchbase://127.0.0.1:1"

var actionTypes = []string{"touch", "exists", "delete", "get", "upsert", "insert", "replace"}

func runCB(t *testing.T, step venom.TestStep) (interface{}, error) {
	t.Helper()
	venom.InitTestLogger(t)
	return Executor{}.Run(context.Background(), step)
}

func oneAction(typ string, extra map[string]any) []any {
	a := map[string]any{"type": typ}
	for k, v := range extra {
		a[k] = v
	}
	return []any{a}
}

func TestSetDefaults(t *testing.T) {
	var e Executor
	e.SetDefaults()
	assert.Equal(t, defaultDSN, e.DSN)
	assert.Equal(t, defaultTranscoder, e.Transcoder)
	assert.InDelta(t, 5.0, e.WaitUntilReadyTimeout, 0.001)
}

func TestFloat64ToDuration(t *testing.T) {
	assert.Equal(t, 1500*time.Millisecond, float64ToDuration(1.5))
	assert.Equal(t, time.Duration(0), float64ToDuration(0))
}

func TestTryGetExpiry(t *testing.T) {
	two, five := 2.0, 5.0
	d, ok := (&Executor{}).tryGetExpiry(nil)
	assert.False(t, ok)
	assert.Equal(t, time.Duration(0), d)

	d, ok = (&Executor{Expiry: &five}).tryGetExpiry(nil)
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, d)

	d, ok = (&Executor{Expiry: &five}).tryGetExpiry(&two)
	assert.True(t, ok)
	assert.Equal(t, 2*time.Second, d)
}

func TestRun_StepDecodeError(t *testing.T) {
	_, err := runCB(t, venom.TestStep{"dsn": []string{"x"}})
	require.Error(t, err)
}

func TestRun_MissingDSN(t *testing.T) {
	_, err := runCB(t, venom.TestStep{"dsn": ""})
	require.ErrorIs(t, err, errMissingCouchbaseDSN)
}

func TestRun_InvalidTranscoder(t *testing.T) {
	_, err := runCB(t, venom.TestStep{"dsn": unreachableDSN, "transcoder": "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid couchbase transcoder "nope"`)
}

func TestRun_TranscodersAndProfileNoActions(t *testing.T) {
	for _, tr := range []string{"json", "raw", "rawjson", "rawstring", "legacy"} {
		got, err := runCB(t, venom.TestStep{
			"dsn": unreachableDSN, "transcoder": tr, "username": "u", "password": "p",
			"profile_wan_development": true,
		})
		require.NoError(t, err, tr)
		res, ok := got.(Result)
		require.True(t, ok)
		assert.Empty(t, res.Actions)
	}
}

func TestRun_UnsupportedAction(t *testing.T) {
	_, err := runCB(t, venom.TestStep{"dsn": unreachableDSN, "actions": oneAction("frobnicate", nil)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `action type "frobnicate" not supported`)
}

func TestRun_ActionDecodeError(t *testing.T) {
	for _, typ := range actionTypes {
		_, err := runCB(t, venom.TestStep{
			"dsn": unreachableDSN, "bucket": "b",
			"actions": oneAction(typ, map[string]any{"ids": 5, "entries": 5}),
		})
		require.Error(t, err, typ)
		assert.Contains(t, err.Error(), "unable to decode", typ)
	}
}

func TestRun_ActionMissingBucket(t *testing.T) {
	for _, typ := range actionTypes {
		_, err := runCB(t, venom.TestStep{
			"dsn": unreachableDSN, "actions": oneAction(typ, nil),
		})
		require.ErrorIs(t, err, errMissingBucketName, typ)
	}
}

func TestRun_TouchMissingExpiry(t *testing.T) {
	_, err := runCB(t, venom.TestStep{
		"dsn": unreachableDSN, "bucket": "b", "wait_until_ready_timeout": 0,
		"actions": oneAction("touch", map[string]any{"ids": []string{"a"}}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing 'expiry'")
}

func TestRun_BucketNotReady(t *testing.T) {
	_, err := runCB(t, venom.TestStep{
		"dsn": unreachableDSN, "bucket": "b", "wait_until_ready_timeout": 0.05,
		"actions": oneAction("exists", map[string]any{"ids": []string{"a"}}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `couchbase bucket "b" not ready`)
}

func TestRun_NoActionsEmptyResult(t *testing.T) {
	got, err := runCB(t, venom.TestStep{"dsn": unreachableDSN, "actions": []any{}})
	require.NoError(t, err)
	assert.Empty(t, actionsOf(t, got))
}

func connectLazy(t *testing.T) *gocb.Cluster {
	t.Helper()
	c, err := gocb.Connect(unreachableDSN, gocb.ClusterOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close(nil) })
	return c
}

func TestGetBucketCachesAndGetCollection(t *testing.T) {
	venom.InitTestLogger(t)
	ctx := context.Background()
	cluster := connectLazy(t)
	e := &Executor{Bucket: "b", Scope: "s", Collection: "c"}

	b1, err := e.getBucket(ctx, cluster, "")
	require.NoError(t, err)
	b2, err := e.getBucket(ctx, cluster, "b")
	require.NoError(t, err)
	assert.Same(t, b1, b2)

	col, err := e.getCollection(ctx, cluster, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "c", col.Name())
	assert.Equal(t, "s", col.ScopeName())

	col, err = e.getCollection(ctx, cluster, "b", "other", "_default")
	require.NoError(t, err)
	assert.Equal(t, "other", col.Name())

	def := &Executor{Bucket: "b"}
	col, err = def.getCollection(ctx, cluster, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "_default", col.Name())

	def.clearCache()
	assert.Empty(t, def.buckets)
}

func TestGetCollectionBucketNotReady(t *testing.T) {
	venom.InitTestLogger(t)
	cluster := connectLazy(t)
	e := &Executor{Bucket: "b", WaitUntilReadyTimeout: 0.05}
	_, err := e.getCollection(context.Background(), cluster, "", "", "")
	require.Error(t, err)
	assert.NotContains(t, e.buckets, "b")
}

func actionsOf(t *testing.T, got interface{}) []any {
	t.Helper()
	res, ok := got.(Result)
	require.True(t, ok)
	return res.Actions
}
