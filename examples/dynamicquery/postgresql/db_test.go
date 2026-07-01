//go:build examples

package dynamicquery

import (
	"context"
	"database/sql"
	"errors"
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
			"INSERT INTO records (tenant_id, name, age, status) VALUES ($1, $2, $3, 'active')",
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
			"INSERT INTO records (tenant_id, name, age, status) VALUES (1, $1, 30, 'active')", n); err != nil {
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

func TestListActiveRecordsDynamic(t *testing.T) {
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
		status string
	}{
		{1, "alice", 30, "active"},
		{1, "bob", 20, "active"},
		{1, "carol", 40, "inactive"},
		{1, "dave", 50, "active"},
		{2, "erin", 35, "active"},
	}
	for _, s := range seed {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO records (tenant_id, name, age, status) VALUES ($1, $2, $3, $4)",
			s.tenant, s.name, s.age, s.status); err != nil {
			t.Fatal(err)
		}
	}

	q := New(db)

	names := func(rows []ListActiveRecordsRow) []string {
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

	active := ListActiveRecordsParams{TenantID: 1, Status: "active"}

	t.Run("no_filters_honors_both_static_predicates", func(t *testing.T) {
		// tenant_id=1 AND status='active' -> alice, bob, dave
		// (carol is inactive; erin is tenant 2)
		got, err := q.ListActiveRecords(ctx, active, ListActiveRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("want 3 rows, got %d (%v)", len(got), names(got))
		}
	})

	t.Run("name_eq_numbers_at_dollar_three", func(t *testing.T) {
		got, err := q.ListActiveRecords(ctx, active, ListActiveRecordsOpts{}.Name("alice"))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"alice"})
	})

	t.Run("age_gte_is_inclusive", func(t *testing.T) {
		// age >= 30 over active tenant-1 rows -> alice(30), dave(50); bob(20) excluded
		got, err := q.ListActiveRecords(ctx, active, ListActiveRecordsOpts{}.Age(30))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 rows, got %d (%v)", len(got), names(got))
		}
	})

	t.Run("combined_dynamic_uses_dollar_three_and_four", func(t *testing.T) {
		got, err := q.ListActiveRecords(ctx, active, ListActiveRecordsOpts{}.Name("dave").Age(30))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"dave"})
	})

	t.Run("order_by_age_desc", func(t *testing.T) {
		got, err := q.ListActiveRecords(ctx, active, ListActiveRecordsOpts{}.OrderBy(ListActiveRecordsOrderByAge, true))
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"dave", "alice", "bob"})
	})

	t.Run("static_status_predicate_isolates", func(t *testing.T) {
		got, err := q.ListActiveRecords(ctx, ListActiveRecordsParams{TenantID: 1, Status: "inactive"}, ListActiveRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"carol"})
	})

	t.Run("static_tenant_predicate_isolates", func(t *testing.T) {
		got, err := q.ListActiveRecords(ctx, ListActiveRecordsParams{TenantID: 2, Status: "active"}, ListActiveRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		eq(t, names(got), []string{"erin"})
	})
}

func TestFilterRecordsDynamic(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ids := make(map[string]int64)
	for _, n := range []string{"alice", "bob", "carol"} {
		var id int64
		if err := db.QueryRowContext(ctx,
			"INSERT INTO records (tenant_id, name, age, status) VALUES (1, $1, 30, 'active') RETURNING id",
			n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[n] = id
	}

	q := New(db)

	t.Run("in with two ids", func(t *testing.T) {
		got, err := q.FilterRecords(ctx, 1, FilterRecordsOpts{}.Ids([]int64{ids["alice"], ids["carol"]}))
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, r := range got {
			names[r.Name] = true
		}
		if len(got) != 2 || !names["alice"] || !names["carol"] {
			t.Fatalf("want [alice carol], got %+v", got)
		}
	})

	t.Run("empty slice applies no IN filter", func(t *testing.T) {
		got, err := q.FilterRecords(ctx, 1, FilterRecordsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("want all 3 rows, got %d", len(got))
		}
	})
}

func TestGetRecordDynamic(t *testing.T) {
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
			"INSERT INTO records (tenant_id, name, age, status) VALUES ($1, $2, $3, 'active')",
			s.tenant, s.name, s.age); err != nil {
			t.Fatal(err)
		}
	}

	q := New(db)

	t.Run("name_eq_returns_single_row", func(t *testing.T) {
		got, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Name("alice"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "alice" || got.Age != 30 {
			t.Fatalf("want alice/30, got %s/%d", got.Name, got.Age)
		}
	})

	t.Run("combined_predicates", func(t *testing.T) {
		got, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Name("carol").Age(40))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "carol" {
			t.Fatalf("want carol, got %s", got.Name)
		}
	})

	t.Run("static_tenant_filter", func(t *testing.T) {
		got, err := q.GetRecord(ctx, 2, GetRecordOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "dave" {
			t.Fatalf("want dave, got %s", got.Name)
		}
	})

	t.Run("no_match_returns_ErrNoRows", func(t *testing.T) {
		_, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Name("nobody"))
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("want sql.ErrNoRows, got %v", err)
		}
	})

	t.Run("order_by_makes_first_row_deterministic", func(t *testing.T) {
		// Multiple tenant-1 rows match age >= 20; ORDER BY age DESC makes the
		// oldest (carol, 40) the deterministic first row QueryRow returns.
		got, err := q.GetRecord(ctx, 1, GetRecordOpts{}.Age(20).OrderBy(GetRecordOrderByAge, true))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "carol" {
			t.Fatalf("want carol (oldest), got %s", got.Name)
		}

		// ASC flips it to the youngest matching row (bob, 20).
		got, err = q.GetRecord(ctx, 1, GetRecordOpts{}.Age(20).OrderBy(GetRecordOrderByAge, false))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "bob" {
			t.Fatalf("want bob (youngest), got %s", got.Name)
		}
	})
}

func TestGetRecordInDynamic(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ids := make(map[string]int64)
	for _, n := range []string{"alice", "bob", "carol"} {
		var id int64
		if err := db.QueryRowContext(ctx,
			"INSERT INTO records (tenant_id, name, age, status) VALUES (1, $1, 30, 'active') RETURNING id",
			n).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[n] = id
	}

	q := New(db)

	t.Run("in_on_single_row_returns_lowest_id", func(t *testing.T) {
		// slice expansion on the :one path; ORDER BY id ASC -> smallest id first.
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

	t.Run("empty_slice_applies_no_IN_filter", func(t *testing.T) {
		// No ids -> no IN predicate; QueryRow yields the first tenant row.
		got, err := q.GetRecordIn(ctx, 1, GetRecordInOpts{}.OrderBy(GetRecordInOrderByID, false))
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != ids["alice"] {
			t.Fatalf("want first tenant row (alice, id %d), got %d", ids["alice"], got.ID)
		}
	})

	t.Run("no_match_returns_ErrNoRows", func(t *testing.T) {
		_, err := q.GetRecordIn(ctx, 1, GetRecordInOpts{}.Ids([]int64{-1}))
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("want sql.ErrNoRows, got %v", err)
		}
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

func TestSearchContactsDynamic(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

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

	t.Run("status_only_emits_bare_leaf", func(t *testing.T) {
		got, err := q.SearchContacts(ctx, 1, SearchContactsOpts{}.Status("inactive"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "bob")
	})

	t.Run("both_set_is_a_disjunction", func(t *testing.T) {
		// (name = alice OR status = inactive): alice by name, bob by status.
		// An AND here would return zero rows, so two rows proves the OR grouping.
		got, err := q.SearchContacts(ctx, 1, SearchContactsOpts{}.Name("alice").Status("inactive"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "alice", "bob")
	})

	t.Run("static_tenant_isolates", func(t *testing.T) {
		got, err := q.SearchContacts(ctx, 2, SearchContactsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "dave")
	})
}

func TestExcludeContactsDynamic(t *testing.T) {
	ctx := context.Background()
	uri := local.PostgreSQL(t, []string{"schema.sql"})

	db, err := sql.Open("pgx", uri)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

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

	t.Run("negate_name", func(t *testing.T) {
		got, err := q.ExcludeContacts(ctx, 1, ExcludeContactsOpts{}.Name("alice"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "bob", "carol")
	})

	t.Run("negate_status", func(t *testing.T) {
		got, err := q.ExcludeContacts(ctx, 1, ExcludeContactsOpts{}.Status("active"))
		if err != nil {
			t.Fatal(err)
		}
		assertNames(t, got, "bob")
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
