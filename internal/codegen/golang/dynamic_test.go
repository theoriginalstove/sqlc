package golang

import (
	"reflect"
	"testing"
)

func TestParseDynamicComments(t *testing.T) {
	tests := []struct {
		name         string
		comments     []string
		wantOps      map[string]string
		wantSort     []string
		wantFiltered []string
	}{
		{
			// The operator is now inferred from the query's WHERE clause
			// (Column.DynamicOp), so parseDynamicComments only records
			// membership; the map values are empty markers.
			name: "bare @dynamic marks membership; trailing op is ignored",
			comments: []string{
				" @dynamic name",       // bare form
				" @dynamic age gt",     // legacy trailing op is ignored
				" @dynamic-sort name, age, created_at",
				" ListRecords lists records",
			},
			wantOps:      map[string]string{"name": "", "age": ""},
			wantSort:     []string{"name", "age", "created_at"},
			wantFiltered: []string{" ListRecords lists records"},
		},
		{
			name: "no dynamic annotations passes comments through untouched",
			comments: []string{
				" GetAuthor returns one author",
			},
			wantOps:      map[string]string{},
			wantSort:     nil,
			wantFiltered: []string{" GetAuthor returns one author"},
		},
		{
			name:         "empty input",
			comments:     nil,
			wantOps:      map[string]string{},
			wantSort:     nil,
			wantFiltered: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, sort, filtered := parseDynamicComments(tt.comments)
			if !reflect.DeepEqual(ops, tt.wantOps) {
				t.Errorf("ops mismatch\n got: %#v\nwant: %#v", ops, tt.wantOps)
			}
			if !reflect.DeepEqual(sort, tt.wantSort) {
				t.Errorf("sort mismatch\n got: %#v\nwant: %#v", sort, tt.wantSort)
			}
			if !reflect.DeepEqual(filtered, tt.wantFiltered) {
				t.Errorf("filtered mismatch\n got: %#v\nwant: %#v", filtered, tt.wantFiltered)
			}
		})
	}
}
