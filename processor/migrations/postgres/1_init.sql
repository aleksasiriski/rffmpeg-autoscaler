CREATE TABLE IF NOT EXISTS hosts (
    "id" SERIAL,
    "servername" TEXT NOT NULL,
    "hostname" TEXT NOT NULL,
    "weight" INTEGER DEFAULT 1,
    "created" TIMESTAMP NOT NULL,
    PRIMARY KEY ("id", "servername")
);
CREATE TABLE IF NOT EXISTS processes (
    "id" SERIAL PRIMARY KEY,
    "host_id" INTEGER,
    "process_id" INTEGER,
    "cmd" TEXT
);
CREATE TABLE IF NOT EXISTS states (
    "id" SERIAL PRIMARY KEY,
    "host_id" INTEGER,
    "process_id" INTEGER,
    "state" TEXT
)