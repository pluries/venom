package venom

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var externalResourceTagRe = regexp.MustCompile(`(?i)<(?:script[^>]*\bsrc|link[^>]*\bhref)\s*=\s*['"](?:https?:)?//[^'"]+['"][^>]*>`)

// Every third-party <script src> or <link href> in the embedded HTML report must carry
// integrity and crossorigin, so the browser refuses a tampered or substituted
// resource instead of loading it silently (SonarQube rule Web:S5725, #1971). Tailwind
// is vendored inline (no CORS support on its CDN, see the comment in venom_output.html)
// and is intentionally not matched here — nothing to verify for a resource never fetched.
func TestOutputHTMLScriptsCarrySubresourceIntegrity(t *testing.T) {
	tags := externalResourceTagRe.FindAllString(templateHTML, -1)
	assert.Len(t, tags, 2, "expected exactly the two CDN-loaded resources (collapse, alpine); "+
		"tailwind is vendored inline and must not appear here, and showdown was removed with Markdown rendering (#2301)")

	for _, tag := range tags {
		assert.Contains(t, tag, `integrity="sha384-`, "missing integrity attribute: %s", tag)
		assert.Contains(t, tag, `crossorigin="anonymous"`, "missing crossorigin attribute: %s", tag)
	}
}

var descriptionElementRe = regexp.MustCompile(`(?s)<!-- Suite Description -->\s*<div[^>]*>`)

// The suite description is test-author text: the report shows it as plain text, never as HTML (T-15, #2301).
func TestOutputHTMLDescriptionIsPlainText(t *testing.T) {
	el := descriptionElementRe.FindString(templateHTML)
	require.NotEmpty(t, el, "the suite description element is missing")
	assert.Contains(t, el, `x-text="test_suites[selectedSuite].description"`)
	assert.Contains(t, el, "whitespace-pre-wrap")
	assert.NotContains(t, templateHTML, "x-html", "no element of the report may render its text as HTML")
	assert.NotContains(t, templateHTML, "showdown", "the Markdown renderer is no longer used")
}
