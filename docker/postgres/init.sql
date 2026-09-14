CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS company_chunks (
    id        bigserial PRIMARY KEY,
    source    text NOT NULL,
    content   text NOT NULL,
    embedding vector(768) NOT NULL
);
