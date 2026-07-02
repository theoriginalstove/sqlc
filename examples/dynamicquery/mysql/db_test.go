//go:build examples

package dynamicquerymysql

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/sqlc-dev/sqlc/internal/sqltest/local"
)

func seedContacts(t *testing.T, ctx context.Context, q *Queries) {
	t.Helper()
	seed := []CreateRecordParams{
		{TenantID: 1, Name: "alice", Age: 30, Status: "active"},
		{TenantID: 1, Name: "bob", Age: 20, Status: "inactive"},
		{TenantID: 1, Name: "carol", Age: 40, Status: "active"},
		{TenantID: 2, Name: "dave", Age: 99, Status: "active"},
	}
	for _, s := range seed {
		if err := q.CreateRecord(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
}

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	uri := local.MySQL(t, []string{"schema.sql"})
	sdb, err := sql.Open("mysql", uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sdb.Close() })
	return sdb
}

func TestListRecordsMySQL(t *testing.T) {
	ctx := context.Background()
	sdb := newDB(t)
	q := New(sdb)
	seedContacts(t, ctx, q)

	t.Run("no_filters_returns_all_tenant_rows", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("want 3 rows, got %d", len(got))
		}
	})

	t.Run("age_gt_uses_question_mark_placeholder", func(t *testing.T) {
		// alice(30), carol(40); bob(20) excluded.
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.Age(25))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 rows, got %d", len(got))
		}
	})

	t.Run("combined_predicates", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.Name("carol").Age(25))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Name != "carol" {
			t.Fatalf("want [carol], got %+v", got)
		}
	})

	t.Run("order_by_age_desc", func(t *testing.T) {
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.OrderBy(ListRecordsOrderByAge, true))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[0].Name != "carol" {
			t.Fatalf("want carol first (desc), got %+v", got)
		}
	})
}

func TestSearchContactsMySQL(t *testing.T) {
	ctx := context.Background()
	sdb := newDB(t)
	q := New(sdb)
	seedContacts(t, ctx, q)

	assertNames := func(t *testing.T, rows []SearchContactsRow, want ...string) {
		t.Helper()
		got := map[string]bool{}
		for _, r := range rows {
			got[r.Name] = true
		}
		if len(got) != len(want) {
			t.Fatalf("got %d rows %v, want %v", len(rows), got, want)
		}
		for _, w := range want {
			if !got[w] {
				t.Fatalf("missing %q in %v", w, got)
			}
		}
	}

	t.Run("no_filters_returns_all_tenant_rows", func(t *testing.T) {
		got, err := q.SearchContacts(ctx, 1, SearchContactsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "alice", "bob", "carol")
	})

	t.Run("both_set_is_a_disjunction", func(t *testing.T) {
		// (name = alice OR status = inactive): alice by name, bob by status.
		// An AND would return zero rows, so two rows proves the OR grouping.
		got, err := q.SearchContacts(ctx, 1, SearchContactsOpts{}.Name("alice").Status("inactive"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "alice", "bob")
	})
}

func TestExcludeContactsMySQL(t *testing.T) {
	ctx := context.Background()
	sdb := newDB(t)
	q := New(sdb)
	seedContacts(t, ctx, q)

	assertNames := func(t *testing.T, rows []ExcludeContactsRow, want ...string) {
		t.Helper()
		got := map[string]bool{}
		for _, r := range rows {
			got[r.Name] = true
		}
		if len(got) != len(want) {
			t.Fatalf("got %d rows %v, want %v", len(rows), got, want)
		}
		for _, w := range want {
			if !got[w] {
				t.Fatalf("missing %q in %v", w, got)
			}
		}
	}

	t.Run("no_filters_omits_the_negated_group", func(t *testing.T) {
		got, err := q.ExcludeContacts(ctx, 1, ExcludeContactsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "alice", "bob", "carol")
	})

	t.Run("negated_disjunction_is_de_morgan", func(t *testing.T) {
		// NOT (name = alice OR status = active) => name != alice AND status != active => bob.
		got, err := q.ExcludeContacts(ctx, 1, ExcludeContactsOpts{}.Name("alice").Status("active"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "bob")
	})
}

func TestFilterRecordsMySQL(t *testing.T) {
	ctx := context.Background()
	sdb := newDB(t)
	q := New(sdb)
	seedContacts(t, ctx, q)

	idByName := func(name string) int64 {
		var id int64
		if err := sdb.QueryRowContext(ctx,
			"SELECT id FROM records WHERE name = ? AND tenant_id = 1", name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	t.Run("in_with_two_ids", func(t *testing.T) {
		got, err := q.FilterRecords(ctx, 1, FilterRecordsOpts{}.Ids([]int64{idByName("alice"), idByName("carol")}))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 rows, got %d", len(got))
		}
	})

	t.Run("empty_slice_applies_no_in_filter", func(t *testing.T) {
		got, err := q.FilterRecords(ctx, 1, FilterRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("want all 3 rows, got %d", len(got))
		}
	})
}
