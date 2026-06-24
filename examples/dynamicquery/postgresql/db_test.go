//go:build examples

package dynamicquery

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sqlc-dev/sqlc/internal/sqltest/local"
)

func TestListRecordsDynamic(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seed := []struct {
		tenant int64
		name   string
		age    int32
	}{
		{1, "alice", 30},
		{1, "bob", 20},
		{1, "carol", 40},
		{2, "dave", 99},
	}
	for _, s := range seed {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO records (tenant_id, name, age) VALUES ($1, $2, $3)",
			s.tenant, s.name, s.age); err != nil {
			t.Fatal(err)
		}
	}

	q := New(db)

	names := func(rows []ListRecordsRow) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = r.Name
		}
		return out
	}
	eq := func(t *testing.T, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}

	t.Run("no_filters_returns_all", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("want 3 rows, got %d (%v)", len(got), names(got))
		}
	})

	t.Run("name_eq", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.Name("alice"))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"alice"})
	})

	t.Run("age_gt", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.Age(25))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 rows, got %d (%v)", len(got), names(got))
		}
	})

	t.Run("combined_predicates", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.Age(25).Name("carol"))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"carol"})
	})

	t.Run("order_by_age_desc", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.OrderBy(ListRecordsOrderByAge, true))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"carol", "alice", "bob"})
	})

	t.Run("static_tenant_filter", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 2, ListRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"dave"})
	})
}

func TestSearchRecordsDynamic(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, n := range []string{"alice", "bob", "carol"} {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO records (tenant_id, name, age) VALUES (1, $1, 30)", n); err != nil {
			t.Fatal(err)
		}
	}

	q := New(db)

	got, err := q.SearchRecords(ctx, 1, SearchRecordsOpts{}.Pattern("car%"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "carol" {
		t.Fatalf(`LIKE "car%%": want [carol], got %+v`, got)
	}
}
