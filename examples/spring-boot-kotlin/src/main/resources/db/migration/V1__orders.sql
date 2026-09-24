CREATE TABLE currencies (
    code VARCHAR(3) PRIMARY KEY,
    name TEXT NOT NULL
);

-- Reference data shipped by the migration. tomato.yml excludes this table
-- from reset, so scenarios never have to re-seed it.
INSERT INTO currencies (code, name) VALUES ('EUR', 'Euro'), ('USD', 'US Dollar');

CREATE TABLE orders (
    id       TEXT PRIMARY KEY,
    amount   BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL REFERENCES currencies (code),
    status   TEXT NOT NULL
);
