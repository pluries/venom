package recipecheck

import (
	yamlv3 "go.yaml.in/yaml/v3"
)

const sizeCap = 1 << 40

// aliasSize is the number of nodes of a document once its aliases are expanded, and how many of them come from an
// alias. Both saturate at sizeCap, so that an alias bomb is counted without being walked.
type aliasSize struct{ total, viaAlias int64 }

type aliasSizer struct {
	memo   map[*yamlv3.Node]aliasSize
	active map[*yamlv3.Node]bool
	cyclic bool
}

func (a *aliasSizer) size(n *yamlv3.Node) aliasSize {
	if n.Kind == yamlv3.AliasNode {
		return a.sizeOfAlias(n)
	}
	if s, ok := a.memo[n]; ok {
		return s
	}
	a.active[n] = true
	s := aliasSize{total: 1}
	for _, c := range n.Content {
		cs := a.size(c)
		s.total = min(s.total+cs.total, sizeCap)
		s.viaAlias = min(s.viaAlias+cs.viaAlias, sizeCap)
	}
	delete(a.active, n)
	a.memo[n] = s
	return s
}

func (a *aliasSizer) sizeOfAlias(n *yamlv3.Node) aliasSize {
	target := n.Alias
	if target == nil || a.active[target] {
		a.cyclic = true
		return aliasSize{total: 1}
	}
	t := a.size(target)
	return aliasSize{total: t.total, viaAlias: t.total}
}

// allowedAliasRatio is yaml.v3's decoder limit on the share of alias-derived nodes, by decoded size.
func allowedAliasRatio(total int64) float64 {
	switch {
	case total <= 400000:
		return 0.99
	case total >= 4000000:
		return 0.10
	}
	return 0.99 - 0.89*float64(total-400000)/float64(4000000-400000)
}

// aliasProblem returns what venom run's decoder would refuse in the aliases of a document: an anchor that contains
// its own alias, or an expansion past yaml.v3's limits. It never walks the expansion.
func aliasProblem(root *yamlv3.Node) string {
	a := &aliasSizer{memo: map[*yamlv3.Node]aliasSize{}, active: map[*yamlv3.Node]bool{}}
	s := a.size(root)
	if a.cyclic {
		return "an anchor contains an alias of itself"
	}
	if s.viaAlias > 100 && s.total > 1000 && float64(s.viaAlias)/float64(s.total) > allowedAliasRatio(s.total) {
		return "document contains excessive aliasing"
	}
	return ""
}
