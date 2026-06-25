-- name: ListRecords :many :dynamic
-- @dynamic name eq
-- @dynamic age gt
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age > sqlc.arg(age);

-- name: ListActiveRecords :many :dynamic
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
