-- name: SearchBad :dynamicmany
-- @dynamic pattern
-- The `~` (POSIX regex) operator has no dynamic mapping, so the operator can't
-- be inferred and codegen must fail loudly rather than emit malformed SQL.
SELECT id, name FROM records
WHERE tenant_id = sqlc.arg(tenant_id)
  AND name ~ sqlc.arg(pattern);
