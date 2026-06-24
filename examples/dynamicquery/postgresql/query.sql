-- name: ListRecords :many :dynamic
-- @dynamic name eq
-- @dynamic age gt
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age > sqlc.arg(age);

-- name: SearchRecords :many :dynamic
-- @dynamic pattern like
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name LIKE sqlc.arg(pattern);

-- name: FilterRecords :many :dynamic
-- @dynamic ids in
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id IN (sqlc.slice(ids));
