-- name: ListRecords :many :dynamic
-- @dynamic name
-- @dynamic age
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age > sqlc.arg(age);
