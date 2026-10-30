package interpolate

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"
)

var interpolateRegex = regexp.MustCompile("({{[\\.\"a-zA-Z0-9._\\-µ|\\s]+}})")

type void struct{}
type val map[string]interface{}

func (v val) Format(s fmt.State, verb rune) {
	if verb == 'v' {
		// s is the fmt.State the fmt package passes into this Format method; writing into its
		// internal buffer during formatting cannot fail.
		_, _ = io.WriteString(s, fmt.Sprintf("%v", v["_"])) //nolint:errcheck // fmt.State write in Format never fails
	}
}

// Do returns interpolated input with vars
func Do(input string, vars map[string]string) (string, error) {
	if !strings.Contains(input, "{{") {
		return input, nil
	}

	data, flatData, replacements, err := indexFor(input, vars)
	if err != nil {
		return "", err
	}
	return render(input, data, flatData, replacements)
}

// render rewrites the "-" in variable references, protects the expressions that name an unknown variable or helper, and
// runs the template on data. Do and DoOver end here, so they cannot drift apart.
func render(input string, data val, flatData map[string]string, replacements []string) (string, error) {
	// handle "-" in var name
	input = strings.NewReplacer(replacements...).Replace(input)
	input = protectUnknownExpressions(input, flatData)
	return execute(input, data)
}

// indexFor indexes the variables input can read, not the whole scope: a suite that holds thousands of variables would
// otherwise pay for all of them on every substitution (#2464). When the template does not parse, or reads the whole data,
// it falls back to indexVariables, so Do reports the same error and gives the same output as with the full index.
func indexFor(input string, vars map[string]string) (val, map[string]string, []string, error) {
	replacements := dashReplacements(vars)
	rewritten := strings.NewReplacer(replacements...).Replace(input)
	names, ok := referencedNames(rewritten)
	if !ok {
		return indexVariables(vars)
	}
	addScannedNames(names, rewritten)
	data, flatData, err := indexSubset(selectVariables(vars, names))
	return data, flatData, replacements, err
}

// referencedNames returns the dotted names a template reads from its data, as spelled after the "-" rewrite. ok is false
// when the template does not parse, or when it can read any variable: a bare "." or "$", or a call to another template.
func referencedNames(input string) (names map[string]void, ok bool) {
	trees, err := template.New("input").Funcs(InterpolateHelperFuncs).Parse(input)
	if err != nil {
		return nil, false
	}
	names = map[string]void{}
	for _, t := range trees.Templates() {
		if t.Tree != nil && !collectNames(t.Root, names) {
			return nil, false
		}
	}
	return names, true
}

// addScannedNames adds the variables protectUnknownExpressions looks up: it splits an expression on spaces, so a "."
// token inside a quoted literal counts as a variable there, though the template does not read it.
func addScannedNames(names map[string]void, input string) {
	for _, sm := range interpolateRegex.FindAllStringSubmatch(input, -1) {
		used, _, _ := scanExpression(strings.TrimSpace(sm[1]))
		for name := range used {
			names[name] = void{}
		}
	}
}

// collectNames adds to names the variables read by n and the nodes under it. It returns false for a node that can read
// any variable.
func collectNames(n parse.Node, names map[string]void) bool {
	switch n := n.(type) {
	case *parse.FieldNode:
		names[strings.Join(n.Ident, ".")] = void{}
	case *parse.VariableNode:
		return collectRootVariable(n, names)
	case *parse.DotNode, *parse.TemplateNode:
		return false
	case *parse.ChainNode:
		return collectNames(n.Node, names)
	case *parse.IfNode:
		return collectBranch(&n.BranchNode, names)
	case *parse.RangeNode:
		return collectBranch(&n.BranchNode, names)
	case *parse.WithNode:
		return collectBranch(&n.BranchNode, names)
	default:
		for _, c := range childNodes(n) {
			if !collectNames(c, names) {
				return false
			}
		}
	}
	return true
}

// collectRootVariable handles "$.name": a field of the root data. A variable the template assigned ("$x") reads nothing
// from the data by itself, and a bare "$" is the whole data.
func collectRootVariable(n *parse.VariableNode, names map[string]void) bool {
	if n.Ident[0] != "$" {
		return true
	}
	if len(n.Ident) == 1 {
		return false
	}
	names[strings.Join(n.Ident[1:], ".")] = void{}
	return true
}

func collectBranch(b *parse.BranchNode, names map[string]void) bool {
	return collectNames(b.Pipe, names) && collectNames(b.List, names) && collectNames(b.ElseList, names)
}

// childNodes returns the nodes directly under n for the node types that hold others; the typed nil pointers that
// text/template leaves in optional places (an absent else, a missing pipe) have none.
func childNodes(n parse.Node) []parse.Node {
	switch n := n.(type) {
	case *parse.ListNode:
		if n != nil {
			return n.Nodes
		}
	case *parse.ActionNode:
		return []parse.Node{n.Pipe}
	case *parse.PipeNode:
		if n != nil {
			cmds := make([]parse.Node, len(n.Cmds))
			for i, c := range n.Cmds {
				cmds[i] = c
			}
			return cmds
		}
	case *parse.CommandNode:
		return n.Args
	}
	return nil
}

// selectVariables keeps the variables a template can reach through names: each name itself, the names above it (a
// field read on a value that also has children) and the names under it (a read on the children of a name).
func selectVariables(vars map[string]string, names map[string]void) map[string]string {
	above, under := reachSets(names)
	selected := make(map[string]string, len(names))
	for k, v := range vars {
		if isReachable(internalName(k), names, above, under) {
			selected[k] = v
		}
	}
	return selected
}

// reachSets returns the names above the read names, and the prefixes of the names under them.
func reachSets(names map[string]void) (above map[string]void, under []string) {
	above = map[string]void{}
	under = make([]string, 0, len(names))
	for name := range names {
		under = append(under, name+".")
		addAncestors(above, name)
	}
	return above, under
}

// addAncestors adds to into the dotted prefixes of name: "a.b" for "a.b.c", then "a".
func addAncestors(into map[string]void, name string) {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			into[name[:i]] = void{}
		}
	}
}

func isReachable(name string, names, above map[string]void, under []string) bool {
	if _, ok := names[name]; ok {
		return true
	}
	if _, ok := above[name]; ok {
		return true
	}
	for _, prefix := range under {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// internalName is how a variable name is spelled inside the template: "-" cannot be part of a field name.
func internalName(k string) string {
	return strings.ReplaceAll(k, "-", "µµµ")
}

// dashReplacements returns the pairs that rewrite the references to the variables whose name has a "-". A name without
// one would be rewritten to itself, so it needs no pair.
func dashReplacements(vars map[string]string) []string {
	var replacements []string
	for k := range vars {
		if strings.Contains(k, "-") {
			kb := internalName(k)
			replacements = append(replacements, "."+k+" ", "."+kb+" ", "."+k+"}", "."+kb+"}", "."+k+"|", "."+kb+"|")
		}
	}
	return replacements
}

// UnknownVariables returns the sorted, de-duplicated names of the variables in input that Do would leave as written:
// the ones that are not in vars, in an expression that has no default or ternary to fall back on. Unknown helpers are
// not reported. Names are given as the user wrote them.
func UnknownVariables(input string, vars map[string]string) ([]string, error) {
	if !strings.Contains(input, "{{") {
		return nil, nil
	}

	_, flatData, replacements, err := indexVariables(vars)
	if err != nil {
		return nil, err
	}
	return unknownIn(input, replacements, func(name string) bool {
		_, known := flatData[name]
		return known
	}), nil
}

// unknownIn is the part of UnknownVariables that reads the template: known says whether a variable, spelled as inside
// the template ("-" as "µµµ"), is in scope. UnknownVariablesOver shares it.
func unknownIn(input string, replacements []string, known func(name string) bool) []string {
	input = strings.NewReplacer(replacements...).Replace(input)

	names := map[string]void{}
	for _, sm := range interpolateRegex.FindAllStringSubmatch(input, -1) {
		usedVariables, usedHelpers, _ := scanExpression(strings.TrimSpace(sm[1]))
		if hasFallback(usedHelpers) {
			continue
		}
		for name := range usedVariables {
			if !known(name) {
				names[strings.ReplaceAll(name, "µµµ", "-")] = void{}
			}
		}
	}
	if len(names) == 0 {
		return nil
	}

	unknown := make([]string, 0, len(names))
	for name := range names {
		unknown = append(unknown, name)
	}
	sort.Strings(unknown)
	return unknown
}

// hasFallback reports whether an expression uses a helper that makes an unknown variable harmless.
func hasFallback(usedHelpers map[string]void) bool {
	_, hasDefault := usedHelpers["default"]
	_, hasTernary := usedHelpers["ternary"]
	return hasDefault || hasTernary
}

// indexVariables nests vars under their dotted names for the template. It also returns the flat index used to
// recognise known variables, and the replacements that rewrite "-" in variable references.
func indexVariables(vars map[string]string) (val, map[string]string, []string, error) {
	data, flatData, err := indexSubset(vars)
	if err != nil {
		return nil, nil, nil, err
	}
	return data, flatData, dashReplacements(vars), nil
}

// indexSubset nests vars under their dotted names, and returns the flat index of the names as spelled in the template.
func indexSubset(vars map[string]string) (val, map[string]string, error) {
	data := make(val, len(vars))
	flatData := make(map[string]string, len(vars))
	for _, k := range sortedByDepth(vars) {
		// handle "-" in var name
		kb := internalName(k)
		if err := setNested(data, k, kb, vars[k]); err != nil {
			return nil, nil, err
		}

		// this is used to check the variables later
		flatData[kb] = vars[k]
	}
	return data, flatData, nil
}

// sortedByDepth returns the names of vars, the most dotted first.
func sortedByDepth(vars map[string]string) []string {
	// sort key, to replace the longer variables before
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.Count(keys[i], ".") > strings.Count(keys[j], ".")
	})
	return keys
}

// setNested stores value in data at the dotted path kb; k is the variable's name as the user wrote it.
func setNested(data val, k, kb, value string) error {
	// Split the keys by dot
	tokens := strings.Split(kb, ".")
	tmp := &data
	for i := 0; i < len(tokens)-1; i++ {
		_, exist := (*tmp)[tokens[i]]
		if !exist {
			(*tmp)[tokens[i]] = &val{}
		}
		next, ok := (*tmp)[tokens[i]].(*val)
		if !ok {
			// A shorter variable name already claimed this path as a leaf value (e.g. "a.b" set
			// before "a.b.c" is processed) — the two variable names conflict.
			return fmt.Errorf("interpolate: variable %q conflicts with a value already set for %q", k, strings.Join(tokens[:i+1], "."))
		}
		tmp = next
	}

	// This is useful to manage {{.cds.env.lb.prefix}}.{{.cds.env.lb}}
	leaf := tokens[len(tokens)-1]
	if existingVal, has := (*tmp)[leaf]; has {
		x, ok := existingVal.(*val)
		if ok {
			(*x)["_"] = value
		}
		(*tmp)[leaf] = x
	} else {
		(*tmp)[leaf] = value
	}
	return nil
}

// protectUnknownExpressions turns each expression that names an unknown variable or helper into a string literal
// of itself, so the template writes it back as it was.
func protectUnknownExpressions(input string, flatData map[string]string) string {
	var processedExpression = map[string]void{}
	for _, sm := range interpolateRegex.FindAllStringSubmatch(input, -1) {
		var expression = strings.TrimSpace(sm[1])
		if _, ok := processedExpression[expression]; ok {
			continue
		}
		processedExpression[expression] = void{}

		usedVariables, usedHelpers, quotedStuff := scanExpression(expression)
		if mustProtect(usedVariables, usedHelpers, flatData) {
			input = quoteExpression(input, sm[1], quotedStuff)
		}
	}
	return input
}

// scanExpression splits an expression into the variables and helpers it names, and its quoted arguments.
func scanExpression(expression string) (usedVariables, usedHelpers map[string]void, quotedStuff []string) {
	usedVariables = map[string]void{}
	usedHelpers = map[string]void{}
	quotedStuff = []string{}

	var trimmedExpression = strings.TrimPrefix(expression, "{{")
	trimmedExpression = strings.TrimSuffix(trimmedExpression, "}}")
	splittedExpression := strings.Split(trimmedExpression, " ")

	// case: {{"conf"|uvault}}
	if len(splittedExpression) == 1 {
		splittedExpression = strings.Split(trimmedExpression, "|")
	}

	for i, s := range splittedExpression {
		splittedExpression[i] = strings.TrimSpace(s)
		if splittedExpression[i] == "" {
			continue
		}

		switch splittedExpression[i][0] {
		case '.':
			usedVariables[splittedExpression[i][1:]] = void{}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			quotedStuff = append(quotedStuff, splittedExpression[i:]...)
		case '"':
			q := strings.TrimPrefix(splittedExpression[i], "\"")
			q = strings.TrimSuffix(q, "\"")
			quotedStuff = append(quotedStuff, q)
		case '|':
		default:
			usedHelpers[splittedExpression[i][0:]] = void{}
		}
	}
	return usedVariables, usedHelpers, quotedStuff
}

// mustProtect reports whether an expression names an unknown variable or helper without a default to fall back on.
func mustProtect(usedVariables, usedHelpers map[string]void, flatData map[string]string) bool {
	// case: {{.assert | ternary "foo" "bar"}} when assert is not defined to avoid {{.assert | default false | ternary "foo" "bar"}}
	if hasFallback(usedHelpers) {
		return false
	}

	for v := range usedVariables {
		if _, is := flatData[v]; !is {
			return true
		}
	}
	for h := range usedHelpers {
		if _, is := InterpolateHelperFuncs[h]; !is {
			return true
		}
	}
	return false
}

// quoteExpression escapes the quoted arguments of expression in input, then wraps every occurrence of it in a
// string literal.
func quoteExpression(input, expression string, quotedStuff []string) string {
	for _, s := range quotedStuff {
		q := strings.ReplaceAll(expression, `"`+s+`"`, `\"`+s+`\"`)
		input = strings.Replace(input, expression, q, 1)
		expression = q
	}
	return strings.ReplaceAll(input, expression, "{{\""+expression+"\"}}")
}

// execute parses input as a template with the helper functions and runs it on data.
func execute(input string, data val) (string, error) {
	t, err := template.New("input").Funcs(InterpolateHelperFuncs).Parse(input)
	if err != nil {
		return "", fmt.Errorf("invalid template format \"%s\": %s", input, err.Error())
	}

	var buff bytes.Buffer
	if err := t.Execute(&buff, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %s", err.Error())
	}

	return buff.String(), nil
}

// Scanned reports whether the unknown-variable protection can read expression, an action with its braces: the scan of
// Do only finds expressions made of the characters of interpolateRegex.
func Scanned(expression string) bool {
	return interpolateRegex.MatchString(expression)
}
