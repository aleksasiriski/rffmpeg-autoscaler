CREATE TABLE hosts (
    id INTEGER PRIMARY KEY,
    hostname TEXT NOT NULL,
    weight INTEGER DEFAULT 1,
    servername TEXT NOT NULL
)
CREATE TABLE processes (
    id INTEGER PRIMARY KEY,
    host_id INTEGER,
    process_id INTEGER,
    cmd TEXT
)
CREATE TABLE states (
    id INTEGER PRIMARY KEY,
    host_id INTEGER,
    process_id INTEGER,
    state TEXT
)