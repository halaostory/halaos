package bot

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
)

// SessionManager manages active chat sessions for bot users.
type SessionManager struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewSessionManager creates a session manager.
func NewSessionManager(queries *store.Queries, logger *slog.Logger) *SessionManager {
	return &SessionManager{queries: queries, logger: logger}
}

// GetOrCreateSession returns the active session ID for a user, creating one if needed.
func (m *SessionManager) GetOrCreateSession(ctx context.Context, identity *UserIdentity, agentSlug string) string {
	if identity.SessionID != "" {
		return identity.SessionID
	}

	// Create new session
	sess, err := m.queries.CreateChatSession(ctx, store.CreateChatSessionParams{
		CompanyID: identity.CompanyID,
		UserID:    identity.UserID,
		AgentSlug: agentSlug,
		Title:     "Telegram Chat",
	})
	if err != nil {
		m.logger.Error("failed to create chat session for bot", "error", err)
		return uuid.New().String()
	}

	sessionID := sess.ID.String()

	// Update the link's active session
	m.queries.UpdateBotUserActiveSession(ctx, store.UpdateBotUserActiveSessionParams{
		ID:              identity.LinkID,
		ActiveSessionID: pgtype.UUID{Bytes: sess.ID, Valid: true},
	})

	identity.SessionID = sessionID
	return sessionID
}

// ResetSession creates a new session and updates the link.
func (m *SessionManager) ResetSession(ctx context.Context, identity *UserIdentity, agentSlug string) string {
	sess, err := m.queries.CreateChatSession(ctx, store.CreateChatSessionParams{
		CompanyID: identity.CompanyID,
		UserID:    identity.UserID,
		AgentSlug: agentSlug,
		Title:     "Telegram Chat",
	})
	if err != nil {
		m.logger.Error("failed to create new chat session", "error", err)
		return ""
	}

	sessionID := sess.ID.String()

	m.queries.UpdateBotUserActiveSession(ctx, store.UpdateBotUserActiveSessionParams{
		ID:              identity.LinkID,
		ActiveSessionID: pgtype.UUID{Bytes: sess.ID, Valid: true},
	})

	identity.SessionID = sessionID
	return sessionID
}
