package venom

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginPathEscapesLib shows the escape #2299 closes: path.Join cleans "../x" out of lib, so the unchecked name
// would load <workdir>/x.so.
func TestPluginPathEscapesLib(t *testing.T) {
	assert.Equal(t, "/work/x.so", path.Join("/work", "lib", "../x"+".so"))
}

func TestValidPluginName(t *testing.T) {
	for _, name := range []string{"foobar", "my-plugin", "my_plugin2"} {
		assert.NoError(t, validPluginName(name), name)
	}
	for _, name := range []string{"../x", "a/b", `a\b`, "..", ".", "", "a\x00b"} {
		assert.Error(t, validPluginName(name), "%q", name)
	}
}

func TestRegisterPluginRefusesAPath(t *testing.T) {
	err := New().registerPlugin(context.Background(), "../x", t.TempDir())
	require.EqualError(t, err, `unable to load plugin "../x": the name must not contain a path`)
}
