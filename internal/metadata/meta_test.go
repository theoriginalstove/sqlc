package metadata

import (
	"reflect"
	"testing"
)

func TestStripDynamicComments(t *testing.T) {
	tests := []struct {
		name     string
		comments []string
		want     []string
	}{
		{
			name: "strips @dynamic and @dynamic-sort, keeps doc line",
			comments: []string{
				" @dynamic name",
				" @dynamic age",
				" @dynamic-sort name, age, created_at",
				" GetRecord returns one record",
			},
			want: []string{" GetRecord returns one record"},
		},
		{
			name: "preserves unrelated @-flags and interleaved doc lines",
			comments: []string{
				" @param foo int",
				" @dynamic name",
				" a doc line",
			},
			want: []string{" @param foo int", " a doc line"},
		},
		{
			name: "no dynamic directives passes through unchanged",
			comments: []string{
				" just a comment",
				" @param x int",
			},
			want: []string{" just a comment", " @param x int"},
		},
		{
			name: "bare @dynamic with no arg is still stripped",
			comments: []string{
				" @dynamic",
				" doc",
			},
			want: []string{" doc"},
		},
		{
			name: "all directives yields empty",
			comments: []string{
				" @dynamic name",
				" @dynamic-sort id",
			},
			want: []string{},
		},
		{
			name:     "empty input yields empty",
			comments: nil,
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripDynamicComments(tt.comments)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("StripDynamicComments mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestParseQueryNameAndType(t *testing.T) {
	for _, query := range []string{
		`-- name: CreateFoo, :one`,
		`-- name: 9Foo_, :one`,
		`-- name: CreateFoo :two`,
		`-- name: CreateFoo`,
		`-- name: CreateFoo :one something`,
		`-- name: `,
		`--name: CreateFoo :one`,
		`--name CreateFoo :one`,
		`--name: CreateFoo :two`,
		"-- name:CreateFoo",
		`--name:CreateFoo :two`,
	} {
		if _, _, _, err := ParseQueryNameAndType(query, CommentSyntax{Dash: true}); err == nil {
			t.Errorf("expected invalid metadata: %q", query)
		}
	}

	for _, query := range []string{
		`-- some comment`,
		`-- name comment`,
		`--name comment`,
	} {
		if _, _, _, err := ParseQueryNameAndType(query, CommentSyntax{Dash: true}); err != nil {
			t.Errorf("expected valid comment: %q", query)
		}
	}

	for query, cs := range map[string]CommentSyntax{
		`-- name: CreateFoo :one`:    {Dash: true},
		`# name: CreateFoo :one`:     {Hash: true},
		`/* name: CreateFoo :one */`: {SlashStar: true},
	} {
		queryName, queryCmd, dynamic, err := ParseQueryNameAndType(query, cs)
		if err != nil {
			t.Errorf("expected valid metadata: %q", query)
		}
		if queryName != "CreateFoo" {
			t.Errorf("incorrect queryName parsed: (%q) %q", queryName, query)
		}
		if queryCmd != CmdOne {
			t.Errorf("incorrect queryCmd parsed: (%q) %q", queryCmd, query)
		}
		if dynamic {
			t.Errorf("incorrectly determined as dynimc query: (%v) %q", dynamic, query)
		}
	}

	for query, want := range map[string]struct {
		cmd string
	}{
		`-- name: ListFoos :dynamicmany`: {cmd: CmdMany},
		`-- name: GetFoo :dynamicone`:    {cmd: CmdOne},
	} {
		queryName, queryCmd, dynamic, err := ParseQueryNameAndType(query, CommentSyntax{Dash: true})
		if err != nil {
			t.Errorf("expected valid metadata: %q", query)
		}
		if queryCmd != want.cmd {
			t.Errorf("incorrect queryCmd parsed: got %q, want %q for %q", queryCmd, want.cmd, query)
		}
		if !dynamic {
			t.Errorf("expected dynamic query: %q", query)
		}
		if err := validateQueryName(queryName); err != nil {
			t.Errorf("unexpected invalid query name %q: %v", queryName, err)
		}
	}

	// a base command followed by a stray token is still rejected as an invalid modifier
	for _, query := range []string{
		`-- name: ListFoos :many :dynamic`,
		`-- name: ListFoos :one :many`,
	} {
		if _, _, _, err := ParseQueryNameAndType(query, CommentSyntax{Dash: true}); err == nil {
			t.Errorf("expected invalid metadata: %q", query)
		}
	}
}

func TestParseQueryParams(t *testing.T) {
	for _, comments := range [][]string{
		{
			" name: CreateFoo :one",
			" @param foo_id UUID",
		},
		{
			" name: CreateFoo :one ",
			" @param foo_id UUID ",
		},
		{
			" name: CreateFoo :one",
			"@param foo_id UUID",
			" invalid",
		},
		{
			" name: CreateFoo :one",
			" @invalid",
			" @param foo_id UUID",
		},
		{
			" name: GetFoos :many ",
			" @param foo_id UUID ",
			" @param @invalid UUID ",
		},
	} {
		params, _, _, _, _, err := ParseCommentFlags(comments)
		if err != nil {
			t.Errorf("expected comments to parse, got err: %s", err)
		}

		pt, ok := params["foo_id"]
		if !ok {
			t.Errorf("expected param not found")
		}

		if pt != "UUID" {
			t.Error("unexpected param metadata:", pt)
		}

		_, ok = params["invalid"]
		if ok {
			t.Errorf("unexpected param found")
		}
	}
}

func TestParseQueryFlags(t *testing.T) {
	for _, comments := range [][]string{
		{
			" name: CreateFoo :one",
			" @flag-foo",
		},
		{
			" name: CreateFoo :one ",
			"@flag-foo ",
		},
		{
			" name: CreateFoo :one",
			" @flag-foo @flag-bar",
		},
		{
			" name: GetFoos :many",
			" @param @flag-bar UUID",
			" @flag-foo",
		},
		{
			" name: GetFoos :many",
			" @flag-foo",
			" @param @flag-bar UUID",
		},
	} {
		_, flags, _, _, _, err := ParseCommentFlags(comments)
		if err != nil {
			t.Errorf("expected comments to parse, got err: %s", err)
		}

		if !flags["@flag-foo"] {
			t.Errorf("expected flag not found")
		}

		if flags["@flag-bar"] {
			t.Errorf("unexpected flag found")
		}
	}
}

func TestParseQueryRuleSkiplist(t *testing.T) {
	for _, comments := range [][]string{
		{
			" name: CreateFoo :one",
			" @sqlc-vet-disable sqlc/db-prepare delete-without-where ",
		},
		{
			" name: CreateFoo :one ",
			" @sqlc-vet-disable sqlc/db-prepare ",
			" @sqlc-vet-disable delete-without-where ",
		},
		{
			" name: CreateFoo :one",
			" @sqlc-vet-disable sqlc/db-prepare ",
			" update-without where",
			" @sqlc-vet-disable delete-without-where ",
		},
	} {
		_, flags, ruleSkiplist, _, _, err := ParseCommentFlags(comments)
		if err != nil {
			t.Errorf("expected comments to parse, got err: %s", err)
		}

		if !flags["@sqlc-vet-disable"] {
			t.Errorf("expected @sqlc-vet-disable flag not found")
		}

		if _, ok := ruleSkiplist["sqlc/db-prepare"]; !ok {
			t.Errorf("expected rule not found in skiplist")
		}

		if _, ok := ruleSkiplist["delete-without-where"]; !ok {
			t.Errorf("expected rule not found in skiplist")
		}

		if _, ok := ruleSkiplist["update-without-where"]; ok {
			t.Errorf("unexpected rule found in skiplist")
		}
	}
}
