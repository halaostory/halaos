-- name: SearchKnowledgeArticles :many
SELECT id, company_id, category, topic, title, content, tags, source, is_active, created_at, updated_at
FROM knowledge_articles
WHERE is_active = true
  AND (company_id IS NULL OR company_id = $1)
  AND search_vector @@ websearch_to_tsquery('english', $2)
ORDER BY ts_rank(search_vector, websearch_to_tsquery('english', $2)) DESC
LIMIT $3;

-- name: SearchKnowledgeArticlesByILIKE :many
SELECT id, company_id, category, topic, title, content, tags, source, is_active, created_at, updated_at
FROM knowledge_articles
WHERE is_active = true
  AND (company_id IS NULL OR company_id = $1)
  AND (title ILIKE '%' || $2 || '%' OR content ILIKE '%' || $2 || '%')
ORDER BY updated_at DESC
LIMIT 3;

-- name: ListKnowledgeArticles :many
SELECT * FROM knowledge_articles
WHERE (company_id IS NULL OR company_id = $1)
  AND ($2::varchar = '' OR category = $2)
ORDER BY category, topic
LIMIT $3 OFFSET $4;

-- name: GetKnowledgeArticle :one
SELECT * FROM knowledge_articles WHERE id = $1;

-- name: CreateKnowledgeArticle :one
INSERT INTO knowledge_articles (company_id, category, topic, title, content, tags, source)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateKnowledgeArticle :one
UPDATE knowledge_articles SET
    category = COALESCE(NULLIF($2, ''), category),
    topic = COALESCE(NULLIF($3, ''), topic),
    title = COALESCE(NULLIF($4, ''), title),
    content = COALESCE(NULLIF($5, ''), content),
    tags = $6,
    source = $7
WHERE id = $1
RETURNING *;

-- name: DeleteKnowledgeArticle :exec
DELETE FROM knowledge_articles WHERE id = $1;

-- name: SearchKnowledgeByTrigram :many
SELECT id, company_id, category, topic, title, content, tags, source, is_active, created_at, updated_at,
       (similarity(title, @query::text) + similarity(content, @query::text))::numeric AS score
FROM knowledge_articles
WHERE is_active = true
  AND (company_id IS NULL OR company_id = @company_id)
  AND (title % @query::text OR content % @query::text)
ORDER BY score DESC
LIMIT @max_results;

-- name: ListKnowledgeCategories :many
SELECT DISTINCT category FROM knowledge_articles
WHERE is_active = true AND (company_id IS NULL OR company_id = $1)
ORDER BY category;
