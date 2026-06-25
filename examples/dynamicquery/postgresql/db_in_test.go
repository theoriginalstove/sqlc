//go:build examples

package dynamicquery

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sqlc-dev/sqlc/internal/sqltest/local"
)

// TestFilterRecordsDynamic exercises a dynamic IN predicate backed by
// sqlc.slice: `id IN (sqlc.slice(ids))`. The builder field is a slice ([]int64,
// not a pointer), and the runtime assembly must expand it into IN ($n, $n+1, …)
// advancing the placeholder counter by len(slice).
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
