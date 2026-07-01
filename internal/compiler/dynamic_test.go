package compiler

import (
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
		// comparison operators pass through unchanged
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
