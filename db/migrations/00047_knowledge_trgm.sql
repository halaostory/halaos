-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_knowledge_title_trgm ON knowledge_articles USING GIN(title gin_trgm_ops);
CREATE INDEX idx_knowledge_content_trgm ON knowledge_articles USING GIN(content gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_knowledge_content_trgm;
DROP INDEX IF EXISTS idx_knowledge_title_trgm;
