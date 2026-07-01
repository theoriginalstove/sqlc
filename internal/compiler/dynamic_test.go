package compiler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sqlc-dev/sqlc/internal/metadata"
	"github.com/sqlc-dev/sqlc/internal/sql/ast"
)

func TestNormalizeDynamicOperator(t *testing.T) {
	tests := []struct {
		op     string
		want   string
		wantOK bool
	}{
		{"=", "=", true},
		{"<", "<", true},
		{">", ">", true},
		{"<=", "<=", true},
		{">=", ">=", true},
		{"<>", "<>", true},
		// Postgres canonicalizes != to <>, accepted defensively
		{"!=", "<>", true},
		// LIKE-family operator spellings normalize to keywords
		{"~~", "LIKE", true},
		{"!~~", "NOT LIKE", true},
		{"~~*", "ILIKE", true},
		{"!~~*", "NOT ILIKE", true},
		// unsupported / unknown tokens
		{"@", "", false},
		{"||", "", false},
		{"", "", false},
		{"@@", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got, ok := normalizeDynamicOperator(tt.op)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("normalizeDynamicOperator(%q) = (%q, %v), want (%q, %v)",
					tt.op, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// opExpr builds an A_Expr of kind OP with the given operator token, e.g. ">=".
func opExpr(op string) *ast.A_Expr {
	return &ast.A_Expr{
		Kind: ast.A_Expr_Kind_OP,
		Name: &ast.List{Items: []ast.Node{&ast.String{Str: op}}},
	}
}

func TestDynamicOperator(t *testing.T) {
	tests := []struct {
		name   string
		expr   *ast.A_Expr
		want   string
		wantOK bool
	}{
		// LIKE / ILIKE / DISTINCT come from the Kind, not the operator token
		{"like from kind", &ast.A_Expr{Kind: ast.A_Expr_Kind_LIKE}, "LIKE", true},
		{"ilike from kind", &ast.A_Expr{Kind: ast.A_Expr_Kind_ILIKE}, "ILIKE", true},
		{"is distinct from", &ast.A_Expr{Kind: ast.A_Expr_Kind_DISTINCT}, "IS DISTINCT FROM", true},
		{"is not distinct from", &ast.A_Expr{Kind: ast.A_Expr_Kind_NOT_DISTINCT}, "IS NOT DISTINCT FROM", true},
		// standard operators flow through the token map
		{"eq from op token", opExpr("="), "=", true},
		{"gte from op token", opExpr(">="), ">=", true},
		{"tilde like from op token", opExpr("~~"), "LIKE", true},
		{"unknown op token", opExpr("&&"), "", false},
		// kinds we don't emit as a simple binary predicate
		{"between unsupported", &ast.A_Expr{Kind: ast.A_Expr_Kind_BETWEEN}, "", false},
		{"in unsupported (handled via slice)", &ast.A_Expr{Kind: ast.A_Expr_Kind_IN}, "", false},
		{"nil expr", nil, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := dynamicOperator(tt.expr)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("dynamicOperator(%s) = (%q, %v), want (%q, %v)",
					tt.name, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func param(number int, name string) Parameter {
	return Parameter{Number: number, Column: &Column{Name: name}}
}

func TestBuildDynamicCodegenSQL(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		params  []Parameter
		md      metadata.Metadata
		want    string
		wantErr bool
	}{
		{
			name: "mixed static and dynamic suffix",
			sql: "SELECT id, name, age, created_at FROM records\n" +
				"WHERE tenant_id = $1\n" +
				"  AND name = $2\n" +
				"  AND age > $3",
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "age")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "eq", "age": "gt"},
			},
			want: "SELECT id, name, age, created_at FROM records\n" +
				"WHERE tenant_id = $1",
		},
		{
			name: "entirely dynamic where clause",
			sql: "SELECT id, name, age FROM records\n" +
				"WHERE name = $1\n" +
				"  AND age > $2",
			params: []Parameter{param(1, "name"), param(2, "age")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "eq", "age": "gt"},
			},
			want: "SELECT id, name, age FROM records",
		},
		{
			name:   "not a dynamic query returns empty",
			sql:    "SELECT id FROM records WHERE tenant_id = $1",
			params: []Parameter{param(1, "tenant_id")},
			md:     metadata.Metadata{},
			want:   "",
		},
		{
			name: "dynamic OR group suffix",
			sql: "SELECT id, name, email FROM records\n" +
				"WHERE tenant_id = $1\n" +
				"  AND (name = $2 OR email = $3)",
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "email")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "", "email": ""},
			},
			want: "SELECT id, name, email FROM records\n" +
				"WHERE tenant_id = $1",
		},
		{
			name: "dynamic OR group then leaf",
			sql: "SELECT id, name, email, age FROM records\n" +
				"WHERE tenant_id = $1\n" +
				"  AND (name = $2 OR email = $3)\n" +
				"  AND age >= $4",
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "email"), param(4, "age")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "", "email": "", "age": ""},
			},
			want: "SELECT id, name, email, age FROM records\n" +
				"WHERE tenant_id = $1",
		},
		{
			name: "dynamic NOT group suffix",
			sql: "SELECT id, name, age FROM records\n" +
				"WHERE tenant_id = $1\n" +
				"  AND NOT (name = $2 AND age > $3)",
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "age")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "", "age": ""},
			},
			want: "SELECT id, name, age FROM records\n" +
				"WHERE tenant_id = $1",
		},
		{
			name: "entirely dynamic OR group",
			sql: "SELECT id, name, email FROM records\n" +
				"WHERE (name = $1 OR email = $2)",
			params: []Parameter{param(1, "name"), param(2, "email")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "", "email": ""},
			},
			want: "SELECT id, name, email FROM records",
		},
		{
			name: "dynamic param before static param is an error",
			sql: "SELECT id FROM records\n" +
				"WHERE name = $1\n" +
				"  AND tenant_id = $2",
			params: []Parameter{param(1, "name"), param(2, "tenant_id")},
			md: metadata.Metadata{
				Dynamic:       true,
				DynamicParams: map[string]string{"name": "eq"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildDynamicCodegenSQL(tt.sql, tt.params, tt.md)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (result %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildDynamicCodegenSQL mismatch\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func cmpLeaf(col string, paramNum int) *ast.A_Expr {
	return &ast.A_Expr{
		Kind:  ast.A_Expr_Kind_OP,
		Name:  &ast.List{Items: []ast.Node{&ast.String{Str: "="}}},
		Lexpr: &ast.ColumnRef{Name: col},
		Rexpr: &ast.ParamRef{Number: paramNum},
	}
}

func boolExpr(op ast.BoolExprType, args ...ast.Node) *ast.BoolExpr {
	return &ast.BoolExpr{Boolop: op, Args: &ast.List{Items: args}}
}

func leafNode(param string) *dynamicNode {
	return &dynamicNode{Param: param}
}

func groupNode(conn string, kids ...*dynamicNode) *dynamicNode {
	return &dynamicNode{Connector: conn, Children: kids}
}

func dumpNode(n *dynamicNode) string {
	if n == nil {
		return "<nil>"
	}
	if n.Connector == "" {
		return n.Param
	}
	parts := make([]string, len(n.Children))
	for i, c := range n.Children {
		parts[i] = dumpNode(c)
	}
	return n.Connector + "(" + strings.Join(parts, ", ") + ")"
}

func assertTree(t *testing.T, got, want *dynamicNode) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildDynamicTree mismatch\n got: %s\nwant: %s",
			dumpNode(got), dumpNode(want))
	}
}

func TestBuildDynamicTree(t *testing.T) {
	dyn := func(names ...string) metadata.Metadata {
		dp := make(map[string]string, len(names))
		for _, n := range names {
			dp[n] = ""
		}
		return metadata.Metadata{Dynamic: true, DynamicParams: dp}
	}

	tests := []struct {
		name    string
		where   ast.Node
		params  []Parameter
		md      metadata.Metadata
		want    *dynamicNode
		wantErr bool
	}{
		{
			name: "flat AND of static + dynamic leaves",
			where: boolExpr(ast.BoolExprTypeAnd,
				cmpLeaf("tenant_id", 1), cmpLeaf("name", 2), cmpLeaf("age", 3)),
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "age")},
			md:     dyn("name", "age"),
			want:   groupNode("AND", leafNode("name"), leafNode("age")),
		},
		{
			name: "static leaf plus a dynamic OR group",
			where: boolExpr(ast.BoolExprTypeAnd,
				cmpLeaf("tenant_id", 1),
				boolExpr(ast.BoolExprTypeOr, cmpLeaf("name", 2), cmpLeaf("email", 3))),
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "email")},
			md:     dyn("name", "email"),
			want: groupNode("AND",
				groupNode("OR", leafNode("name"), leafNode("email"))),
		},
		{
			name: "dynamic OR group followed by a dynamic leaf",
			where: boolExpr(ast.BoolExprTypeAnd,
				cmpLeaf("tenant_id", 1),
				boolExpr(ast.BoolExprTypeOr, cmpLeaf("name", 2), cmpLeaf("email", 3)),
				cmpLeaf("age", 4)),
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "email"), param(4, "age")},
			md:     dyn("name", "email", "age"),
			want: groupNode("AND",
				groupNode("OR", leafNode("name"), leafNode("email")),
				leafNode("age")),
		},
		{
			name: "NOT wrapping a nested dynamic AND group",
			where: boolExpr(ast.BoolExprTypeAnd,
				cmpLeaf("tenant_id", 1),
				boolExpr(ast.BoolExprTypeNot,
					boolExpr(ast.BoolExprTypeAnd, cmpLeaf("name", 2), cmpLeaf("age", 3)))),
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "age")},
			md:     dyn("name", "age"),
			want: groupNode("AND",
				groupNode("NOT",
					groupNode("AND", leafNode("name"), leafNode("age")))),
		},
		{
			name: "entirely dynamic top-level OR group",
			where: boolExpr(ast.BoolExprTypeOr,
				cmpLeaf("name", 1), cmpLeaf("email", 2)),
			params: []Parameter{param(1, "name"), param(2, "email")},
			md:     dyn("name", "email"),
			want: groupNode("AND",
				groupNode("OR", leafNode("name"), leafNode("email"))),
		},
		{
			name: "deeply nested dynamic groups",
			where: boolExpr(ast.BoolExprTypeAnd,
				cmpLeaf("tenant_id", 1),
				boolExpr(ast.BoolExprTypeOr,
					cmpLeaf("name", 2),
					boolExpr(ast.BoolExprTypeAnd, cmpLeaf("min_age", 3), cmpLeaf("max_age", 4)))),
			params: []Parameter{param(1, "tenant_id"), param(2, "name"), param(3, "min_age"), param(4, "max_age")},
			md:     dyn("name", "min_age", "max_age"),
			want: groupNode("AND",
				groupNode("OR",
					leafNode("name"),
					groupNode("AND", leafNode("min_age"), leafNode("max_age")))),
		},
		{
			name: "NOT wrapping a single dynamic leaf",
			where: boolExpr(ast.BoolExprTypeAnd,
				cmpLeaf("tenant_id", 1),
				boolExpr(ast.BoolExprTypeNot, cmpLeaf("name", 2))),
			params: []Parameter{param(1, "tenant_id"), param(2, "name")},
			md:     dyn("name"),
			want:   groupNode("AND", groupNode("NOT", leafNode("name"))),
		},
		{
			name: "fully static nested group is dropped",
			where: boolExpr(ast.BoolExprTypeAnd,
				boolExpr(ast.BoolExprTypeAnd, cmpLeaf("tenant_id", 1), cmpLeaf("status", 2)),
				cmpLeaf("name", 3)),
			params: []Parameter{param(1, "tenant_id"), param(2, "status"), param(3, "name")},
			md:     dyn("name"),
			want:   groupNode("AND", leafNode("name")),
		},
		{
			name:   "single dynamic leaf as the whole where",
			where:  cmpLeaf("name", 1),
			params: []Parameter{param(1, "name")},
			md:     dyn("name"),
			want:   groupNode("AND", leafNode("name")),
		},
		{
			name:   "non-dynamic query yields a nil tree",
			where:  boolExpr(ast.BoolExprTypeAnd, cmpLeaf("tenant_id", 1)),
			params: []Parameter{param(1, "tenant_id")},
			md:     metadata.Metadata{},
			want:   nil,
		},
		{
			name: "mixed static and dynamic in one group is an error",
			where: boolExpr(ast.BoolExprTypeOr,
				cmpLeaf("tenant_id", 1), cmpLeaf("name", 2)),
			params:  []Parameter{param(1, "tenant_id"), param(2, "name")},
			md:      dyn("name"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildDynamicTree(tt.where, tt.params, tt.md)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (tree %+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertTree(t, got, tt.want)
		})
	}
}
