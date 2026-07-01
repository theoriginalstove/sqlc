package golang

import (
	"fmt"

	"github.com/sqlc-dev/sqlc/internal/plugin"
)

type DynamicQuery struct {
	StaticCount int
	Opts        []DynamicPredicate
	Steps       []DynamicStep
	SortColumns []DynamicSortColumn
}

type DynamicPredicate struct {
	FieldName string
	VarName   string
	GoType    string
	Column    string
	SQLOp     string
	IsSlice   bool
}

type DynamicSortColumn struct {
	ConstName string
	Value     string
}

type DynamicStep struct {
	Kind      string
	Predicate *DynamicPredicate
	Target    string
	SliceVar  string
	Connector string
	Not       bool
	Cap       int
}

func flattenDynamic(n *plugin.DynamicNode, predicates map[string]DynamicPredicate, target string, counter *int) []DynamicStep {
	if n.GetConnector() == "" {
		predicate := predicates[n.GetParam()]
		return []DynamicStep{{Kind: "leaf", Predicate: &predicate, Target: target}}
	}
	sliceVar := fmt.Sprintf("g%d", *counter)
	*counter++
	steps := []DynamicStep{{Kind: "open", SliceVar: sliceVar, Cap: len(n.GetChildren())}}
	for _, c := range n.GetChildren() {
		steps = append(steps, flattenDynamic(c, predicates, sliceVar, counter)...)
	}
	connector, not := n.GetConnector(), false
	if connector == "NOT" {
		connector, not = "AND", true
	}
	return append(steps, DynamicStep{Kind: "close", SliceVar: sliceVar, Target: target, Connector: connector, Not: not})
}
