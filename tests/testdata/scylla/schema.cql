-- Schema for the scylladb integration feature. Runs once at startup, after
-- tomato creates the `tomato` keyspace.
CREATE TABLE IF NOT EXISTS tomato.users (
    id int PRIMARY KEY,
    name text,
    email text,
    active boolean,
    tags list<text>
);

CREATE TABLE IF NOT EXISTS tomato.countries (
    code text PRIMARY KEY,
    name text
);

-- Reference data, kept across scenarios via `exclude`.
INSERT INTO tomato.countries (code, name) VALUES ('DE', 'Germany');
INSERT INTO tomato.countries (code, name) VALUES ('FR', 'France');
