package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/metadata"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
	"github.com/sqlc-dev/sqlc/internal/sql/astutils"
)

func dynamicOperator(n *ast.A_Expr) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind {
	case ast.A_Expr_Kind_LIKE:
		return "LIKE", true
	case ast.A_Expr_Kind_ILIKE:
		return "ILIKE", true
	case ast.A_Expr_Kind_DISTINCT:
		return "IS DISTINCT FROM", true
	case ast.A_Expr_Kind_NOT_DISTINCT:
		return "IS NOT DISTINCT FROM", true
	case ast.A_Expr_Kind_OP:
		return normalizeDynamicOperator(astutils.Join(n.Name, "."))
	default:
		return "", false
	}
}

func normalizeDynamicOperator(op string) (string, bool) {
	switch op {
	case "=", "<", ">", "<=", ">=", "<>":
		return op, true
	case "!=":
		return "<>", true
	case "~~":
		return "LIKE", true
	case "!~~":
		return "NOT LIKE", true
	case "~~*":
		return "ILIKE", true
	case "!~~*":
		return "NOT ILIKE", true
	default:
		return "", false
	}
}

func buildDynamicCodegenSQL(sql string, params []Parameter, md metadata.Metadata) (string, error) {
	if !md.Dynamic || len(md.DynamicParams) == 0 {
		return "", nil
	}

	staticCount := 0
	for _, p := range params {
		if p.Column == nil {
			continue
		}
		if _, ok := md.DynamicParams[p.Column.Name]; !ok {
			staticCount++
		}
	}

	for _, p := range params {
		if p.Column == nil {
			continue
		}
		if _, ok := md.DynamicParams[p.Column.Name]; ok && p.Number <= staticCount {
			return "", fmt.Errorf("dynamic param %q must appear after all static parameters", p.Column.Name)
		}
	}

	firstDyn := staticCount + 1
	idx := placeholderIndex(sql, firstDyn)
	if idx < 0 {
		return "", fmt.Errorf("dynamic: couldn't locaTe placeholder $%d", firstDyn)
	}

	connector := "AND"
	if staticCount == 0 {
		connector = "WHERE"
	}
	cut := lastKeyword(sql[:idx], connector)
	if cut < 0 {
		return "", fmt.Errorf("dynamic: couldn't locate %q before $%d", connector, firstDyn)
	}
	return strings.TrimRight(sql[:cut], " \t\n"), nil
}

func placeholderIndex(sql string, n int) int {
	needle := "$" + strconv.Itoa(n)
	for from := 0; ; {
		i := strings.Index(sql[from:], needle)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(needle)
		if end >= len(sql) || sql[end] < '0' || sql[end] > '9' {
			return i
		}
		from = end
	}
}

func lastKeyword(s, kw string) int {
	lower, lkw := strings.ToLower(s), strings.ToLower(kw)
	for from := len(lower); ; {
		i := strings.LastIndex(lower[:from], lkw)
		if i < 0 {
			return -1
		}
		beforeOK := i == 0 || isSpace(lower[i-1])
		after := i + len(lkw)
		aftreOK := after >= len(lower) || isSpace(lower[after])
		if beforeOK && aftreOK {
			return i
		}
		from = i
	}
}

func isSpace(b byte) bool {
	return b == ' ' ||
		b == '\t' ||
		b == '\n' ||
		b == '\r'
}

type DynamicNode struct {
	Connector string
	Param     string
	Children  []*DynamicNode
}

func buildDynamicTree(where ast.Node, params []Parameter, md metadata.Metadata) (*DynamicNode, error) {
	if !md.Dynamic || len(md.DynamicParams) == 0 {
		return nil, nil
	}
	numName := make(map[int]string, len(params))
	dynNum := make(map[int]bool, len(params))

	for _, p := range params {
		if p.Column == nil {
			continue
		}

		numName[p.Number] = p.Column.Name
		if _, ok := md.DynamicParams[p.Column.Name]; ok {
			dynNum[p.Number] = true
		}
	}

	terms := []ast.Node{where}
	if boolExpr, ok := where.(*ast.BoolExpr); ok && boolExpr.Boolop == ast.BoolExprTypeAnd {
		terms = boolExpr.Args.Items
	}

	root := &DynamicNode{Connector: "AND"}
	for _, term := range terms {
		node, isDyn, err := classifyDynamic(term, numName, dynNum)
		if err != nil {
			return nil, err
		}
		if isDyn {
			root.Children = append(root.Children, node)
		}
	}
	return root, nil
}

func classifyDynamic(n ast.Node, numName map[int]string, dynNum map[int]bool) (*DynamicNode, bool, error) {
	if boolExpr, ok := n.(*ast.BoolExpr); ok {
		switch boolExpr.Boolop {
		case ast.BoolExprTypeNot:
			var inner ast.Node
			if boolExpr.Args != nil && len(boolExpr.Args.Items) > 0 {
				inner = boolExpr.Args.Items[0]
			}
			child, isDyn, err := classifyDynamic(inner, numName, dynNum)
			if err != nil || !isDyn {
				return nil, false, err
			}
			return &DynamicNode{Connector: "NOT", Children: []*DynamicNode{child}}, true, nil
		case ast.BoolExprTypeAnd, ast.BoolExprTypeOr:
			connector := "AND"
			if boolExpr.Boolop == ast.BoolExprTypeOr {
				connector = "OR"
			}
			var children []*DynamicNode
			var anyStatic, anyDynamic bool
			for _, arg := range boolExpr.Args.Items {
				child, isDyn, err := classifyDynamic(arg, numName, dynNum)
				if err != nil {
					return nil, false, err
				}
				if isDyn {
					anyDynamic = true
					children = append(children, child)
				} else {
					anyStatic = true
				}
			}
			switch {
			case anyStatic && anyDynamic:
				return nil, false, fmt.Errorf("dynamic: a group must be entirely dynamic (mixes static and dynamic)")
			case anyDynamic:
				return &DynamicNode{Connector: connector, Children: children}, true, nil
			default:
				// fully static group to stay in the const
				return nil, false, nil
			}
		}
	}
	num, ok := leafParamNumber(n)
	if !ok || !dynNum[num] {
		return nil, false, nil
	}
	return &DynamicNode{Param: numName[num]}, true, nil
}

func leafParamNumber(n ast.Node) (int, bool) {
	if n == nil {
		return 0, false
	}
	refs := astutils.Search(n, func(node ast.Node) bool {
		_, ok := node.(*ast.ParamRef)
		return ok
	})
	if refs == nil || len(refs.Items) == 0 {
		return 0, false
	}
	pr, ok := refs.Items[0].(*ast.ParamRef)
	if !ok {
		return 0, false
	}
	return pr.Number, true
}
