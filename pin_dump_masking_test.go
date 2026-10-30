package venom

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gosimple/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheDumpFilesOfARunHideTheDeclaredSecret pins the dump files a -vv run writes (RULE-026): neither their content nor
// their names carry a declared secret in any of its forms, while the replacement token is there.
func TestTheDumpFilesOfARunHideTheDeclaredSecret(t *testing.T) {
	v, _ := suiteNewVenom(t, 2)
	v.OutputDir = t.TempDir()
	ts, _ := suiteForRunTestCases(t, v, guardSuiteYAML(true))

	v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

	files, err := filepath.Glob(filepath.Join(v.OutputDir, "*.dump.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "a -vv run writes one dump file per step that ran")

	forms := []string{guardSecret, slug.Make(guardSecret), url.QueryEscape(guardSecret), url.PathEscape(guardSecret)}
	hidden := false
	for _, f := range files {
		content, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, form := range forms {
			assert.NotContains(t, f, form, "the file name carries the secret")
			assert.NotContains(t, string(content), form, "%s carries the secret", f)
		}
		hidden = hidden || strings.Contains(string(content), "__hidden__")
	}
	assert.True(t, hidden, "at least one dump file shows the replacement token")
}
