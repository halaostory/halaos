package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/halaostory/halaos/internal/store"
)

// Linker handles bot user identity resolution and link code flows.
type Linker struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewLinker creates a linker service.
func NewLinker(queries *store.Queries, logger *slog.Logger) *Linker {
	return &Linker{queries: queries, logger: logger}
}

// ResolveIdentity looks up a linked user by platform + platform user ID.
func (l *Linker) ResolveIdentity(ctx context.Context, platform, platformUserID string) (*UserIdentity, error) {
	link, err := l.queries.GetBotUserLinkByPlatformUser(ctx, store.GetBotUserLinkByPlatformUserParams{
		Platform:       platform,
		PlatformUserID: &platformUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("user not linked: %w", err)
	}

	var sessionID string
	if link.ActiveSessionID.Valid {
		sessionID = uuid.UUID(link.ActiveSessionID.Bytes).String()
	}

	var employeeID int64
	if link.EmployeeID != nil {
		employeeID = *link.EmployeeID
	}

	return &UserIdentity{
		LinkID:     link.ID,
		UserID:     link.UserID,
		CompanyID:  link.CompanyID,
		EmployeeID: employeeID,
		FirstName:  deref(link.FirstName),
		LastName:   deref(link.LastName),
		Locale:     link.Locale,
		SessionID:  sessionID,
	}, nil
}

// VerifyLinkCode verifies a link code and associates the platform user.
func (l *Linker) VerifyLinkCode(ctx context.Context, code, platform, platformUserID string) (*UserIdentity, error) {
	link, err := l.queries.GetBotUserLinkByCode(ctx, &code)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired link code")
	}

	verified, err := l.queries.VerifyBotUserLink(ctx, store.VerifyBotUserLinkParams{
		ID:             link.ID,
		PlatformUserID: &platformUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("verify link: %w", err)
	}

	l.logger.Info("bot user linked",
		"user_id", verified.UserID,
		"platform", platform,
		"platform_user_id", platformUserID,
	)

	return &UserIdentity{
		LinkID:    verified.ID,
		UserID:    verified.UserID,
		CompanyID: verified.CompanyID,
	}, nil
}

// GenerateLinkCode creates a new link code for a user.
func (l *Linker) GenerateLinkCode(ctx context.Context, userID, companyID int64) (string, error) {
	code := generateCode()

	// Check if link already exists
	existing, err := l.queries.GetBotUserLinkByUserID(ctx, store.GetBotUserLinkByUserIDParams{
		UserID:   userID,
		Platform: "telegram",
	})
	if err == nil {
		// Link exists, regenerate code
		_, err := l.queries.RegenerateLinkCode(ctx, store.RegenerateLinkCodeParams{
			UserID:   existing.UserID,
			LinkCode: &code,
		})
		if err != nil {
			return "", fmt.Errorf("regenerate link code: %w", err)
		}
		return code, nil
	}

	// Create new link
	_, err = l.queries.CreateBotUserLink(ctx, store.CreateBotUserLinkParams{
		Platform:  "telegram",
		UserID:    userID,
		CompanyID: companyID,
		LinkCode:  &code,
	})
	if err != nil {
		return "", fmt.Errorf("create link: %w", err)
	}

	return code, nil
}

func generateCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
