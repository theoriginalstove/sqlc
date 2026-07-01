CREATE TABLE records (
    id         BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id  BIGINT NOT NULL,
    name       VARCHAR(255) NOT NULL,
    age        INT NOT NULL,
    status     VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
