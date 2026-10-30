package venom

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"

	"github.com/pkg/errors"
)

//go:embed venom_output.html
var templateHTML string

type TestsHTML struct {
	Tests     Tests       `json:"tests"`
	JSONValue template.JS `json:"jsonValue"`
}

func outputHTML(testsResult *Tests) ([]byte, error) {
	var buf bytes.Buffer

	testJSON, err := json.MarshalIndent(testsResult, "", " ")
	if err != nil {
		return nil, errors.Wrap(err, "unable to make json value")
	}

	testsHTML := TestsHTML{
		Tests: *testsResult,
		//nolint:gosec // json.MarshalIndent HTML-escapes <, > and & by default, which already neutralises the </script> breakout this cast would otherwise reintroduce
		JSONValue: template.JS(testJSON),
	}
	tmpl := template.Must(template.New("reportHTML").Parse(templateHTML))
	if err := tmpl.Execute(&buf, testsHTML); err != nil {
		return nil, errors.Wrap(err, "unable to make template")
	}
	return buf.Bytes(), nil
}
