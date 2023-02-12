CREATE TABLE hosts (
    id SERIAL PRIMARY KEY,
    hostname TEXT NOT NULL,
    weight INTEGER DEFAULT 1,
    servername TEXT NOT NULL
)
CREATE TABLE processes (
    id SERIAL PRIMARY KEY,
    host_id INTEGER,
    process_id INTEGER,
    cmd TEXT
)
CREATE TABLE states (
    id SERIAL PRIMARY KEY,
    host_id INTEGER,
    process_id INTEGER,
    state TEXT
)