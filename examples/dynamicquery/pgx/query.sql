-- name: ListRecords :dynamicmany
-- @dynamic name eq
-- @dynamic age gt
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age > sqlc.arg(age);

-- name: SearchRecords :dynamicmany
-- @dynamic pattern like
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name LIKE sqlc.arg(pattern);

-- name: FilterRecords :dynamicmany
-- @dynamic ids in
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id IN (sqlc.slice(ids));

-- name: GetRecord :dynamicone
-- GetRecord returns a single tenant record, optionally narrowed by an exact
-- name and a minimum age, and optionally ordered. QueryRow yields the first
-- matching row (order it to make "first" deterministic).
-- @dynamic name eq
-- @dynamic age gte
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age >= sqlc.arg(age);

-- name: ListActiveRecords :dynamicmany
-- ListActiveRecords returns a tenant's records for a given status, optionally
-- narrowed by an exact name and a minimum age, and optionally ordered.
-- @dynamic name eq
-- @dynamic age gte
-- @dynamic-sort name, age, created_at
SELECT id, name, age, status, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = sqlc.arg(status)
  AND name = sqlc.arg(name)
  AND age >= sqlc.arg(age);

-- name: GetRecordIn :dynamicone
-- GetRecordIn returns the first tenant record whose id is in an optional set,
-- exercising sqlc.slice IN-expansion on the single-row path. Order it to make
-- "first" deterministic.
-- @dynamic ids in
-- @dynamic-sort id
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id IN (sqlc.slice(ids));
