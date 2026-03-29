package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/ai"
	"github.com/halaostory/halaos/internal/ai/agent"
	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/billing"
	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/store"
)

// processAgentTasks picks up pending agent tasks and executes them via the AI executor.
// Called every event-loop cycle (5s). Max 5 tasks per batch.
func processAgentTasks(ctx context.Context, cfg *config.Config, queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) {
	if !cfg.AI.Enabled {
		return
	}

	// Resolve AI provider
	var aiProvider provider.Provider
	switch {
	case cfg.AI.AnthropicKey != "":
		aiProvider = provider.NewAnthropic(cfg.AI.AnthropicKey, "")
	case cfg.AI.OpenAIKey != "":
		aiProvider = provider.NewOpenAI(cfg.AI.OpenAIKey, "")
	default:
		return
	}

	// Fetch pending tasks (max 5 per cycle)
	tasks, err := queries.GetPendingAgentTasks(ctx, 5)
	if err != nil {
		logger.Error("failed to get pending agent tasks", "error", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	logger.Info("processing agent tasks", "count", len(tasks))

	// Build executor (lazy — only when tasks exist)
	billingSvc := billing.NewService(queries, logger)
	toolRegistry := ai.NewToolRegistry(queries, pool)
	agentRegistry := agent.NewRegistry(queries, logger)
	executor := agent.NewExecutor(aiProvider, toolRegistry, billingSvc, agentRegistry, queries, logger, nil, nil)

	for _, task := range tasks {
		// Claim the task (atomic CAS: pending → running)
		claimed, err := queries.ClaimAgentTask(ctx, task.ID)
		if err != nil {
			logger.Warn("could not claim agent task", "task_id", task.ID, "error", err)
			continue
		}

		executeAgentTask(ctx, executor, queries, claimed, logger)
	}
}

func executeAgentTask(ctx context.Context, executor *agent.Executor, queries *store.Queries, task store.AgentTask, logger *slog.Logger) {
	taskCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logger.Info("executing agent task",
		"task_id", task.ID,
		"agent", task.AgentSlug,
		"company_id", task.CompanyID,
		"user_id", task.UserID,
	)

	resp, err := executor.Chat(taskCtx, task.CompanyID, task.UserID, task.AgentSlug, agent.ChatRequest{
		Message: task.Input,
	})

	if err != nil {
		errMsg := err.Error()
		_ = queries.UpdateAgentTask(ctx, store.UpdateAgentTaskParams{
			ID:             task.ID,
			Status:         "failed",
			ErrorMessage:   &errMsg,
			TokensConsumed: 0,
		})
		logger.Error("agent task failed", "task_id", task.ID, "error", err)
		return
	}

	output := resp.Message
	_ = queries.UpdateAgentTask(ctx, store.UpdateAgentTaskParams{
		ID:             task.ID,
		Status:         "completed",
		Output:         &output,
		TokensConsumed: resp.TokensUsed,
	})

	logger.Info("agent task completed",
		"task_id", task.ID,
		"tokens", resp.TokensUsed,
		"agent", resp.Agent,
	)

	// Notify the user that their task is done
	preview := output
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	title := fmt.Sprintf("AI Task Completed: %s", task.AgentSlug)
	entityType := "agent_task"
	notification.Notify(ctx, queries, logger, task.CompanyID, task.UserID,
		title, preview, "ai_task", &entityType, &task.ID)
}
