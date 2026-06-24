package compiler

import (
	"testing"

	"github.com/sqlc-dev/sqlc/internal/metadata"
)

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
