-- name: ListAgents :many
SELECT * FROM agents WHERE is_active = true ORDER BY slug;

-- name: ListAgentsByCompany :many
SELECT * FROM agents
WHERE is_active = true
  AND (company_id IS NULL OR company_id = $1)
ORDER BY company_id NULLS FIRST, slug;

-- name: GetAgentBySlug :one
SELECT * FROM agents WHERE slug = $1 AND is_active = true;

-- name: CreateAgent :one
INSERT INTO agents (company_id, slug, name, description, system_prompt, tools,
  cost_multiplier, is_autonomous, max_rounds, max_tokens, icon, model)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateAgent :one
UPDATE agents SET
  name = COALESCE(NULLIF(@name::varchar, ''), name),
  description = COALESCE(NULLIF(@description::text, ''), description),
  system_prompt = COALESCE(NULLIF(@system_prompt::text, ''), system_prompt),
  tools = @tools,
  cost_multiplier = @cost_multiplier,
  is_autonomous = @is_autonomous,
  max_rounds = @max_rounds,
  max_tokens = @max_tokens,
  icon = COALESCE(NULLIF(@icon::varchar, ''), icon),
  model = @model,
  updated_at = now()
WHERE slug = @slug AND is_active = true
RETURNING *;

-- name: DeactivateAgent :exec
UPDATE agents SET is_active = false, updated_at = now()
WHERE slug = $1 AND company_id = $2;

-- name: CreateAgentTask :one
INSERT INTO agent_tasks (company_id, user_id, agent_slug, status, input)
VALUES ($1, $2, $3, 'pending', $4)
RETURNING *;

-- name: UpdateAgentTask :exec
UPDATE agent_tasks
SET status = $2,
    output = $3,
    tokens_consumed = $4,
    error_message = $5,
    started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN now() ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('completed', 'failed') THEN now() ELSE completed_at END
WHERE id = $1;

-- name: ListAgentTasks :many
SELECT * FROM agent_tasks
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPendingAgentTasks :many
SELECT * FROM agent_tasks
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: ClaimAgentTask :one
UPDATE agent_tasks
SET status = 'running', started_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;
