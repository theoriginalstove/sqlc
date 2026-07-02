-- name: ListRecords :dynamicmany
-- @dynamic name
-- @dynamic age
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age > sqlc.arg(age);

-- name: ListActiveRecords :dynamicmany
-- ListActiveRecords returns a tenant's records for a given status, optionally
-- narrowed by an exact name and a minimum age, and optionally ordered.
-- @dynamic name
-- @dynamic age
-- @dynamic-sort name, age, created_at
SELECT id, name, age, status, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = sqlc.arg(status)
  AND name = sqlc.arg(name)
  AND age >= sqlc.arg(age);

-- name: GetRecord :dynamicone
-- GetRecord returns a single tenant record, optionally narrowed by an exact
-- name and a minimum age. QueryRow yields the first matching row.
-- @dynamic name
-- @dynamic age
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age >= sqlc.arg(age);

-- name: SearchContacts :dynamicmany
-- @dynamic name
-- @dynamic status
SELECT id, name, age, status, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (name = sqlc.arg(name) OR status = sqlc.arg(status));

-- name: ExcludeContacts :dynamicmany
-- @dynamic name
-- @dynamic status
SELECT id, name, age, status, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND NOT (name = sqlc.arg(name) OR status = sqlc.arg(status));
