package recipecheck

import (
	yamlv3 "go.yaml.in/yaml/v3"
)

// Suite is a suite file read as YAML nodes.
type Suite struct {
	Path string
	Name string
	// Spec is the value of the suite variable "spec", empty when the suite does not name one.
	Spec string
	// Text is the whole text of the file, for the rules that scan it (RC04).
	Text      string
	Node      *yamlv3.Node
	Testcases []*Testcase
}

// Testcase is a testcase of a suite. Steps holds only the steps whose type is web or mobile.
type Testcase struct {
	Name string
	Line int
	// Skipped is true when the testcase has a "skip" key.
	Skipped bool
	Node    *yamlv3.Node
	Steps   []*Step
}

// Step is a web or mobile step.
type Step struct {
	Type       string
	Line       int
	Node       *yamlv3.Node
	Assertions []Assertion
}

// Assertion is one assertion string of a step with the line of its node.
type Assertion struct {
	Text string
	Line int
}

// parseSuite reads one suite file without interpolating it. It returns the suite, or an RC10 finding when the YAML
// does not parse or "testcases" is present but is not a list. A file with no "testcases" is a suite with none.
func parseSuite(path string, text []byte) (*Suite, *Finding) {
	var doc yamlv3.Node
	if err := yamlv3.Unmarshal(text, &doc); err != nil {
		return nil, rc10(path, 0, "the file does not parse: "+err.Error())
	}
	suite := &Suite{Path: path, Text: string(text)}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return suite, nil
	}
	if msg := aliasProblem(doc.Content[0]); msg != "" {
		return nil, rc10(path, doc.Content[0].Line, msg)
	}
	root := deref(doc.Content[0])
	if root.Kind != yamlv3.MappingNode {
		return nil, rc10(path, root.Line, "the file is not a mapping")
	}
	if line, msg := unreadable(root); msg != "" {
		return nil, rc10(path, line, msg)
	}
	suite.Node = root
	suite.Name = scalarOf(mapValue(root, "name"))
	if vars := mapValue(root, "vars"); vars != nil {
		suite.Spec = scalarOf(mapValue(vars, "spec"))
	}
	key, value := mapEntry(root, "testcases")
	if key == nil || isNull(value) {
		return suite, nil
	}
	if value.Kind != yamlv3.SequenceNode {
		return nil, rc10(path, key.Line, "testcases is present but is not a list")
	}
	for _, n := range value.Content {
		n = deref(n)
		if isNull(n) {
			continue
		}
		if n.Kind != yamlv3.MappingNode {
			return nil, rc10(path, n.Line, "the testcase is not a mapping")
		}
		suite.Testcases = append(suite.Testcases, readTestcase(n))
	}
	return suite, nil
}

// isNull is true for an empty value or a ~ / null scalar, which venom run reads as absent.
func isNull(n *yamlv3.Node) bool {
	return n != nil && n.Kind == yamlv3.ScalarNode && n.ShortTag() == "!!null"
}

// unreadable finds what venom run rejects and the YAML parser lets through: a duplicate key, and a step that is
// not a mapping. It returns the line and the reason, or an empty reason.
func unreadable(root *yamlv3.Node) (int, string) {
	if key := duplicateKey(root); key != nil {
		return key.Line, "duplicate key " + key.Value
	}
	if step := stepNotAMapping(root); step != nil {
		return step.Line, "the step is not a mapping"
	}
	return 0, ""
}

// duplicateKey returns the second occurrence of a key repeated in a mapping, at any depth. Aliases are not followed:
// their anchor was walked where it is defined.
func duplicateKey(n *yamlv3.Node) *yamlv3.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yamlv3.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Value != "<<" && seen[k.Value] {
				return k
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if dup := duplicateKey(c); dup != nil {
			return dup
		}
	}
	return nil
}

// stepNotAMapping returns the first entry of a testcase's steps list that is not a mapping.
func stepNotAMapping(root *yamlv3.Node) *yamlv3.Node {
	for _, tc := range sequenceItems(mapValue(root, "testcases")) {
		for _, step := range sequenceItems(mapValue(deref(tc), "steps")) {
			if step = deref(step); step.Kind != yamlv3.MappingNode && !isNull(step) {
				return step
			}
		}
	}
	return nil
}

func sequenceItems(n *yamlv3.Node) []*yamlv3.Node {
	if n == nil || n.Kind != yamlv3.SequenceNode {
		return nil
	}
	return n.Content
}

// walkScalars calls visit for every scalar value under n (mapping keys excluded), following aliases.
func walkScalars(n *yamlv3.Node, visit func(*yamlv3.Node)) {
	n = deref(n)
	if n == nil {
		return
	}
	switch n.Kind {
	case yamlv3.ScalarNode:
		visit(n)
	case yamlv3.SequenceNode:
		for _, c := range n.Content {
			walkScalars(c, visit)
		}
	case yamlv3.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			walkScalars(n.Content[i], visit)
		}
	}
}

// visitScalars calls visit for the scalars of the variable defaults of every suite, testcase and step, and for the
// scalars under the roots that stepRoots returns for each web or mobile step.
func visitScalars(f *Folder, stepRoots func(*Step) []*yamlv3.Node, visit func(*Suite, *Testcase, *yamlv3.Node)) {
	for _, s := range f.Suites {
		walkScalars(mapValue(s.Node, "vars"), func(n *yamlv3.Node) { visit(s, nil, n) })
		for _, tc := range s.Testcases {
			walkScalars(mapValue(tc.Node, "vars"), func(n *yamlv3.Node) { visit(s, tc, n) })
			for _, st := range tc.Steps {
				for _, root := range stepRoots(st) {
					walkScalars(root, func(n *yamlv3.Node) { visit(s, tc, n) })
				}
			}
		}
	}
}

func rc10(path string, line int, msg string) *Finding {
	return &Finding{File: path, Line: line, Rule: "RC10", Message: msg}
}

func readTestcase(n *yamlv3.Node) *Testcase {
	skip, _ := mapEntry(n, "skip")
	tc := &Testcase{Name: scalarOf(mapValue(n, "name")), Line: n.Line, Skipped: skip != nil, Node: n}
	if steps := mapValue(n, "steps"); steps != nil && steps.Kind == yamlv3.SequenceNode {
		for _, s := range steps.Content {
			if step := readStep(deref(s)); step != nil {
				tc.Steps = append(tc.Steps, step)
			}
		}
	}
	return tc
}

func readStep(n *yamlv3.Node) *Step {
	if n.Kind != yamlv3.MappingNode {
		return nil
	}
	typ := scalarOf(mapValue(n, "type"))
	if typ != "web" && typ != "mobile" {
		return nil
	}
	step := &Step{Type: typ, Line: n.Line, Node: n}
	if list := mapValue(n, "assertions"); list != nil && list.Kind == yamlv3.SequenceNode {
		for _, a := range list.Content {
			a = deref(a)
			if a.Kind == yamlv3.ScalarNode {
				step.Assertions = append(step.Assertions, Assertion{Text: a.Value, Line: a.Line})
			}
		}
	}
	return step
}

// deref follows an alias to the node it stands for.
func deref(n *yamlv3.Node) *yamlv3.Node {
	for n != nil && n.Kind == yamlv3.AliasNode {
		n = n.Alias
	}
	return n
}

// mapEntry returns the key and value nodes of a mapping key, or nil nodes when the key is absent.
func mapEntry(m *yamlv3.Node, key string) (*yamlv3.Node, *yamlv3.Node) {
	if m == nil || m.Kind != yamlv3.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], deref(m.Content[i+1])
		}
	}
	return nil, nil
}

func mapValue(m *yamlv3.Node, key string) *yamlv3.Node {
	_, v := mapEntry(m, key)
	return v
}

func scalarOf(n *yamlv3.Node) string {
	if n == nil || n.Kind != yamlv3.ScalarNode {
		return ""
	}
	return n.Value
}
