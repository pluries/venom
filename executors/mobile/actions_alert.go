package mobile

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// The alert action (FR-12, #2398): answers the system alert that an iOS app raised, for instance a permission
// request. Android has no such action. The alert text goes to result.text and nowhere else: it can hold a secret, so
// no log line and no error text carries it.

// Values of the step field choice.
const (
	choiceAccept  = "accept"
	choiceDismiss = "dismiss"
)

// errNoAlert is the failure of an alert step when no alert is open.
var errNoAlert = errors.New("no alert open")

// alertChoice reads the step's choice: accept (the default) or dismiss, in any case.
func alertChoice(step Executor) (string, error) {
	switch choice := strings.ToLower(step.Choice); choice {
	case "", choiceAccept:
		return choiceAccept, nil
	case choiceDismiss:
		return choiceDismiss, nil
	default:
		return "", usagef("choice must be %s or %s, got %q", choiceAccept, choiceDismiss, step.Choice)
	}
}

// isNoSuchAlert tells the answer of a server that has no alert open.
func isNoSuchAlert(err error) bool {
	var failure *serverError
	return errors.As(err, &failure) && failure.Code == "no such alert"
}

// noAlertOr maps "no such alert" to errNoAlert and leaves any other error as it is.
func noAlertOr(err error) error {
	if isNoSuchAlert(err) {
		return errNoAlert
	}
	return err
}

func actAlert(ctx context.Context, s *session, step Executor, result *Result) error {
	if !s.ios() {
		return usagef("alert is iOS only: Android has no system alert action")
	}
	choice, err := alertChoice(step)
	if err != nil {
		return err
	}
	raw, err := s.call(ctx, http.MethodGet, "/alert/text", nil)
	if err != nil {
		return noAlertOr(err)
	}
	if string(raw) != "null" {
		if result.Text, err = textOf(raw); err != nil {
			return err
		}
	}
	return noAlertOr(answerAlert(ctx, s, choice, step.Button))
}

// answerAlert accepts or dismisses the open alert: by button label with mobile: alert, else with the W3C command.
func answerAlert(ctx context.Context, s *session, choice, button string) error {
	if button != "" {
		return iosExecute(ctx, s, "mobile: alert", map[string]interface{}{"action": choice, "buttonLabel": button})
	}
	_, err := s.call(ctx, http.MethodPost, "/alert/"+choice, emptyBody())
	return err
}
