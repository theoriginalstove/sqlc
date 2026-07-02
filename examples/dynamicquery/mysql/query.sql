-- name: ListRecords :dynamicmany
-- @dynamic name
-- @dynamic age
-- @dynamic-sort name, age, created_at
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name = sqlc.arg(name)
  AND age > sqlc.arg(age);

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

-- name: FilterRecords :dynamicmany
-- @dynamic ids
SELECT id, name, age, created_at FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND id IN (sqlc.slice(ids));

-- name: CreateRecord :exec
INSERT INTO records (tenant_id, name, age, status)
VALUES (sqlc.arg(tenant_id), sqlc.arg(name), sqlc.arg(age), sqlc.arg(status));
