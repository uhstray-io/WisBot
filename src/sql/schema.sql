
-- name: CreateFilesTable :exec
CREATE TABLE IF NOT EXISTS files (
    Id               TEXT PRIMARY KEY NOT NULL,
    Discord_Username TEXT NOT NULL,
    Name             TEXT NOT NULL,
    Data             BYTEA,
    Size             INTEGER DEFAULT 0 NOT NULL,
    Downloads        INTEGER DEFAULT 0 NOT NULL,

    Uploaded         BOOLEAN NOT NULL,
    Created_At       TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
  );
