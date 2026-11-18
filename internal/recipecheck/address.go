package recipecheck

import (
	"regexp"
	"strings"

	yamlv3 "go.yaml.in/yaml/v3"
)

var (
	schemeURL = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9+.-]*://|file:)`)
	httpHost  = regexp.MustCompile(`(?i)\bhttps?://([^\s/?#'"\\]*)`)
)

// hostOf is the host of an authority as net/url's Hostname reads it: the userinfo up to the last "@" is dropped, then
// the port, and a bracketed IPv6 address keeps its brackets.
func hostOf(authority string) string {
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		authority = authority[i+1:]
	}
	if strings.HasPrefix(authority, "[") {
		if j := strings.Index(authority, "]"); j >= 0 {
			return authority[:j+1]
		}
		return authority
	}
	host, _, _ := strings.Cut(authority, ":")
	return host
}

// isLocalHost is the contract's local host: localhost, 127.0.0.1, ::1, *.localhost, *.test, *.invalid and empty.
func isLocalHost(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	switch h {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".test") || strings.HasSuffix(h, ".invalid")
}

// nonLocalHost returns the first http or https host of s that is neither local nor a template.
func nonLocalHost(s string) (string, bool) {
	for _, m := range httpHost.FindAllStringSubmatch(s, -1) {
		if host := hostOf(m[1]); !strings.Contains(host, "{{") && !isLocalHost(host) {
			return host, true
		}
	}
	return "", false
}

// stepAddressRoots are the parts of a step whose http hosts must be local: the execute script, the mock bodies and
// the step's variable defaults.
func stepAddressRoots(st *Step) []*yamlv3.Node {
	roots := []*yamlv3.Node{mapValue(st.Node, "vars"), mapValue(mapValue(st.Node, "action"), "execute")}
	for _, mock := range sequenceItems(mapValue(st.Node, "mock")) {
		roots = append(roots, mapValue(deref(mock), "body"))
	}
	return roots
}

// navigateURL returns the url node of a navigate action: its url key, or the value itself when it is a scalar.
func navigateURL(st *Step) *yamlv3.Node {
	nav := mapValue(mapValue(st.Node, "action"), "navigate")
	if nav != nil && nav.Kind == yamlv3.ScalarNode {
		return nav
	}
	return mapValue(nav, "url")
}

// authorityHasTemplate reports whether the authority of a url (between "://" and the first "/", "?" or "#") holds a
// template; a url without "://" is read whole.
func authorityHasTemplate(url string) bool {
	_, rest, found := strings.Cut(url, "://")
	if !found {
		return strings.Contains(url, "{{")
	}
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		end = len(rest)
	}
	return strings.Contains(rest[:end], "{{")
}

func fixedNavigations(s *Suite, tc *Testcase) []Finding {
	var out []Finding
	for _, st := range tc.Steps {
		u := navigateURL(st)
		if u != nil && schemeURL.MatchString(u.Value) && !authorityHasTemplate(u.Value) {
			out = append(out, s.finding(tc, "RC09", u.Line, "navigate has a fixed address: take it from a variable"))
		}
	}
	return out
}

// ruleRC09 reports a navigate url with a scheme and no template, and a non-local http or https host in an execute
// script, a mock body or a variable default.
func ruleRC09(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		for _, tc := range s.Testcases {
			out = append(out, fixedNavigations(s, tc)...)
		}
	}
	visitScalars(f, stepAddressRoots, func(s *Suite, tc *Testcase, n *yamlv3.Node) {
		if host, bad := nonLocalHost(n.Value); bad {
			out = append(out, s.finding(tc, "RC09", n.Line, "the host "+host+" is not local: the recipe must not reach a real server"))
		}
	})
	return out
}
