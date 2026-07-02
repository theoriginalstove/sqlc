CREATE TABLE records (
    id        BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name      TEXT NOT NULL
);
