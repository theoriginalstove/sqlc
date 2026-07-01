CREATE TABLE records (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id  INTEGER NOT NULL,
    name       TEXT NOT NULL,
    age        INTEGER NOT NULL,
    status     TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
