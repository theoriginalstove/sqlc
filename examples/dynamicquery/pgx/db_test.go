//go:build examples

package dynamicquerypgx

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/sqlc-dev/sqlc/internal/sqltest/local"
)

func TestDynamicPgx(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

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
	ids := make(map[string]int64)
	for _, s := range seed {
		var id int64
		if err := db.QueryRow(ctx,
			"INSERT INTO records (tenant_id, name, age, status) VALUES ($1, $2, $3, 'active') RETURNING id",
			s.tenant, s.name, s.age).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[s.name] = id
	}

	q := New(db)

	// :dynamicmany through the native pgx driver.
	t.Run("many_filter_and_order", func(t *testing.T) {
		// ListRecords: @dynamic age gt -> age > 25 over tenant 1 -> alice(30), carol(40);
		// ORDER BY age DESC -> carol, alice.
		got, err := q.ListRecords(ctx, 1, ListRecordsOpts{}.Age(25).OrderBy(ListRecordsOrderByAge, true))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "carol" || got[1].Name != "alice" {
			t.Fatalf("want [carol alice], got %+v", got)
		}
	})

	// :dynamicone through the native pgx driver.
	t.Run("one_name_eq", func(t *testing.T) {
		got, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Name("alice"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "alice" || got.Age != 30 {
			t.Fatalf("want alice/30, got %s/%d", got.Name, got.Age)
		}
	})

	t.Run("one_order_makes_first_row_deterministic", func(t *testing.T) {
		got, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Age(20).OrderBy(GetRecordOrderByAge, true))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "carol" {
			t.Fatalf("want carol (oldest), got %s", got.Name)
		}
	})

	t.Run("one_no_match_returns_ErrNoRows", func(t *testing.T) {
		_, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Name("nobody"))
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("want pgx.ErrNoRows, got %v", err)
		}
	})

	// IN-expansion on the :dynamicone path through the native pgx driver.
	t.Run("in_on_single_row_returns_lowest_id", func(t *testing.T) {
		got, err := q.GetRecordIn(ctx, 1, GetRecordInOpts{}.
			Ids([]int64{ids["carol"], ids["alice"]}).
			OrderBy(GetRecordInOrderByID, false))
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != ids["alice"] {
			t.Fatalf("want alice's id %d (lowest), got %d", ids["alice"], got.ID)
		}
	})

	t.Run("in_empty_slice_applies_no_filter", func(t *testing.T) {
		got, err := q.GetRecordIn(ctx, 1, GetRecordInOpts{}.OrderBy(GetRecordInOrderByID, false))
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != ids["alice"] {
			t.Fatalf("want first tenant row (alice, id %d), got %d", ids["alice"], got.ID)
		}
	})
}

func TestSearchContactsPgx(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	q := New(db)
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

	t.Run("name_only_emits_bare_leaf", func(t *testing.T) {
		got, err := q.SearchContacts(ctx, 1, SearchContactsOpts{}.Name("alice"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "alice")
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

func TestExcludeContactsPgx(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	q := New(db)
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
