package mobile

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ovh/venom"
)

// The files a failing step leaves in the process's working directory (FR-14, FR-15). A passing step writes and deletes
// none; the next failing step replaces the pair. Two runs sharing one working directory overwrite each other's files:
// a documented limit (README).
const (
	diagnosticScreenshot = "screen.png"
	diagnosticSource     = "screen.dump.xml"
)

// diagnosticsTimeout bounds each of the two requests of the diagnostics. A package variable so that tests can shorten it.
var diagnosticsTimeout = 15 * time.Second

// diagnose saves a screenshot and the view hierarchy after a failed step. It is best effort: a failure to fetch or write
// one file is a warning, never a change to the step's own failure, and does not stop the other file. It runs on the
// session's context, not the step's (the step may have timed out), so closing the session stops it.
// The caller holds runMu.
func (s *session) diagnose(ctx context.Context, failure error) {
	if s.base.Err() != nil || sessionIsGone(failure) {
		return
	}
	s.saveDiagnostic(ctx, "/screenshot", diagnosticScreenshot, decodeScreenshot)
	s.saveDiagnostic(ctx, "/source", diagnosticSource, decodeSource)
}

// sessionIsGone tells a failure caused by a session the server no longer has: there is nothing to look at.
func sessionIsGone(err error) bool {
	var failure *serverError
	return errors.As(err, &failure) && (failure.Code == "invalid session id" || failure.Code == "no such session")
}

// saveDiagnostic fetches one resource of the session and writes it, owner-only, to file.
func (s *session) saveDiagnostic(ctx context.Context, path, file string, decode func([]byte) ([]byte, error)) {
	raw, err := s.c.do(s.base, http.MethodGet, "/session/"+s.id+path, nil, diagnosticsTimeout)
	if err == nil {
		var data []byte
		if data, err = decode(raw); err == nil {
			err = writeOwnerOnly(file, data)
		}
	}
	if err != nil {
		venom.Warn(ctx, "mobile diagnostics: %s not saved: %v", file, err)
	}
}

// decodeScreenshot turns the value of GET /screenshot into the PNG bytes, within the size cap (T8).
func decodeScreenshot(raw []byte) ([]byte, error) {
	encoded, err := textOf(raw)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("the screenshot is not valid base64")
	}
	if len(data) == 0 {
		return nil, errors.New("the automation server returned an empty screenshot")
	}
	if len(data) > maxScreenshotBytes {
		return nil, errors.New("the screenshot is larger than the allowed size")
	}
	return data, nil
}

// decodeSource turns the value of GET /source into the file content.
func decodeSource(raw []byte) ([]byte, error) {
	text, err := textOf(raw)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, errors.New("the automation server returned an empty page source")
	}
	return []byte(text), nil
}
