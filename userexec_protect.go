package venom

import (
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/ovh/venom/interpolate"
)

// The body of a custom executor is interpolated once with the executor's own scope, and its steps again, as JSON, when
// they run. An expression that names a variable the first pass does not have must reach the second pass as written.
// interpolate leaves such an expression alone only when it is made of letters, digits and a few signs; an action with a
// format verb or a parenthesis, {{ printf "%s" .name }}, was rendered at once with an empty value (#3305).

// actionPattern finds an action with its braces.
var actionPattern = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// stringLiteral is a double-quoted string without escapes.
var stringLiteral = regexp.MustCompile(`"([^"\` + "`" + `]*)"`)

// protectLaterActions returns body with each action that reads a variable has does not know, and that interpolate does not
// protect itself, rewritten as an action that renders to the original text, with its string literals as raw strings so that
// the JSON of the step holds no \" in it. The body is left as it is, whole, unless it is only text and independent
// actions: nothing with a template variable, a block, a comment or a trim marker, and nothing that does not parse.
func protectLaterActions(body string, has func(name string) bool) string {
	if !strings.Contains(body, "{{") {
		return body
	}
	tree, err := template.New("body").Funcs(interpolate.InterpolateHelperFuncs).Parse(body)
	if err != nil || tree.Tree == nil || !independentActions(tree.Root) {
		return body
	}
	found := actionPattern.FindAllString(body, -1)
	if len(found) != len(tree.Root.Nodes)-textNodes(tree.Root) {
		return body
	}
	return actionPattern.ReplaceAllStringFunc(body, func(action string) string {
		return protectAction(action, has)
	})
}

// independentActions reports whether the list holds only text and actions that do not depend on one another.
func independentActions(list *parse.ListNode) bool {
	for _, n := range list.Nodes {
		switch n := n.(type) {
		case *parse.TextNode:
		case *parse.ActionNode:
			if len(n.Pipe.Decl) != 0 || strings.Contains(n.String(), "$") {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// textNodes counts the text nodes of the list.
func textNodes(list *parse.ListNode) int {
	count := 0
	for _, n := range list.Nodes {
		if n.Type() == parse.NodeText {
			count++
		}
	}
	return count
}

// protectAction returns the action that renders to action when it reads an unknown variable, or action itself.
func protectAction(action string, has func(name string) bool) string {
	if interpolate.Scanned(action) || strings.Contains(action, "\n") || strings.HasPrefix(action, "{{-") || strings.HasSuffix(action, "-}}") {
		return action
	}
	if !readsUnknownVariable(action, has) {
		return action
	}
	raw := stringLiteral.ReplaceAllString(action, "`$1`")
	if strings.ContainsAny(raw, `"\`) {
		return action
	}
	return `{{"` + raw + `"}}`
}

// readsUnknownVariable reports whether the action reads a variable has does not know and has no default or ternary,
// which make an unknown variable harmless.
func readsUnknownVariable(action string, has func(name string) bool) bool {
	tree, err := template.New("action").Funcs(interpolate.InterpolateHelperFuncs).Parse(action)
	if err != nil || tree.Tree == nil {
		return false
	}
	names, idents := map[string]struct{}{}, map[string]struct{}{}
	for _, n := range tree.Root.Nodes {
		if a, ok := n.(*parse.ActionNode); ok {
			scanPipe(a.Pipe, names, idents)
		}
	}
	if usesAny(idents, "default", "ternary") || usesAny(idents, quoteEmitters...) || emitsQuoteVerb(action) {
		return false
	}
	for name := range names {
		if !has(name) {
			return true
		}
	}
	return false
}

// scanPipe collects the dotted variable names and the function names a pipeline uses, those of its parenthesised
// pipelines included.
func scanPipe(p *parse.PipeNode, names, idents map[string]struct{}) {
	for _, c := range p.Cmds {
		for _, arg := range c.Args {
			switch a := arg.(type) {
			case *parse.FieldNode:
				names[strings.Join(a.Ident, ".")] = struct{}{}
			case *parse.IdentifierNode:
				idents[a.Ident] = struct{}{}
			case *parse.PipeNode:
				scanPipe(a, names, idents)
			}
		}
	}
}

// quoteEmitters are the functions whose output can hold a double quote or a line break, which would break the JSON of
// the step once the action is rendered into it; an action that calls one is left as it was.
var quoteEmitters = []string{"quote", "stringQuote", "toJSON", "toPrettyJSON", "escape", "toString", "indent", "nindent"}

// usesAny reports whether the pipeline calls one of the functions.
func usesAny(idents map[string]struct{}, names ...string) bool {
	for _, name := range names {
		if _, ok := idents[name]; ok {
			return true
		}
	}
	return false
}

// emitsQuoteVerb reports whether a format of the action prints its argument with quotes.
func emitsQuoteVerb(action string) bool {
	return strings.Contains(action, "%q") || strings.Contains(action, "%#")
}
