package mobile

import (
	"context"
	"net/http"
)

// maxLinkBytes caps the length of a deep link.
const maxLinkBytes = 2048

// checkLink refuses a deep link that is not scheme://rest, carries a control character or a space, or is too long. It
// is a misuse, raised before any request. The value stays out of the message: a link can carry a token.
func checkLink(link string) error {
	if link == "" {
		return usagef("openURL needs the step field url")
	}
	if len(link) > maxLinkBytes {
		return usagef("openURL: the url is longer than %d bytes", maxLinkBytes)
	}
	for i := 0; i < len(link); i++ {
		if c := link[i]; c <= ' ' || c == 0x7F {
			return usagef("openURL: the url must not contain spaces or control characters")
		}
	}
	if !hasSchemeAndRest(link) {
		return usagef("openURL: the url must look like scheme://...")
	}
	return nil
}

// hasSchemeAndRest tells whether link is an RFC 3986 scheme (a letter, then letters, digits, "+", "-" or ".")
// followed by "://" and at least one more character.
func hasSchemeAndRest(link string) bool {
	i := 0
	for ; i < len(link); i++ {
		c := link[i]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if letter {
			continue
		}
		if i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.') {
			continue
		}
		break
	}
	if i == 0 || len(link) <= i+3 {
		return false
	}
	return link[i:i+3] == "://"
}

// actOpenURL opens a deep link in the app (FR-09): mobile: deepLink, restricted to the app package when it is known.
// No scheme is refused, custom app schemes are the point; the link is only sent to the automation server.
func actOpenURL(ctx context.Context, s *session, step Executor, _ *Result) error {
	if s.ios() {
		return iosOpenURL(ctx, s, step)
	}
	if err := checkLink(step.URL); err != nil {
		return err
	}
	args := map[string]interface{}{"url": step.URL}
	if appID, err := appOf(s, step); err == nil {
		args["package"] = appID
	}
	_, err := s.call(ctx, http.MethodPost, "/execute/sync", executeBody("mobile: deepLink", args))
	return err
}
