package byok

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/integration/crypto"
	"github.com/tonypk/aigonhr/internal/store"
)

// Resolver resolves the best available LLM provider for a given company/user.
// Priority: User BYOK → Company BYOK → Platform default.
type Resolver struct {
	queries          *store.Queries
	encryptor        *crypto.CredentialEncryptor
	defaultProvider  provider.Provider
	defaultAnthropic string // platform Anthropic key
	defaultOpenAI    string // platform OpenAI key
	defaultGemini    string // platform Gemini key
	logger           *slog.Logger
}

// NewResolver creates a BYOK key resolver.
func NewResolver(
	queries *store.Queries,
	encryptor *crypto.CredentialEncryptor,
	defaultProvider provider.Provider,
	anthropicKey, openaiKey, geminiKey string,
	logger *slog.Logger,
) *Resolver {
	return &Resolver{
		queries:          queries,
		encryptor:        encryptor,
		defaultProvider:  defaultProvider,
		defaultAnthropic: anthropicKey,
		defaultOpenAI:    openaiKey,
		defaultGemini:    geminiKey,
		logger:           logger,
	}
}

// ResolvedProvider holds the resolved provider and metadata.
type ResolvedProvider struct {
	Provider provider.Provider
	IsBYOK   bool   // true if using customer's own key
	Source   string // "user_byok", "company_byok", "platform"
}

// Resolve finds the best provider for the given company and user.
// It checks BYOK keys first, then falls back to the platform default.
func (r *Resolver) Resolve(ctx context.Context, companyID, userID int64) ResolvedProvider {
	if r.encryptor == nil {
		return ResolvedProvider{Provider: r.defaultProvider, Source: "platform"}
	}

	// Try to find a BYOK key — we check each provider the platform supports.
	for _, providerName := range []string{"anthropic", "openai", "gemini"} {
		byokKey, err := r.queries.ResolveByokKey(ctx, store.ResolveByokKeyParams{
			CompanyID: companyID,
			Provider:  providerName,
			UserID:    &userID,
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			r.logger.Warn("byok resolve error", "provider", providerName, "err", err)
			continue
		}

		// Decrypt the key
		plaintext, err := r.encryptor.Decrypt(byokKey.EncryptedKey)
		if err != nil {
			r.logger.Error("byok decrypt error", "id", byokKey.ID, "err", err)
			continue
		}

		apiKey := string(plaintext)
		model := byokKey.ModelOverride

		source := "company_byok"
		if byokKey.UserID != nil {
			source = "user_byok"
		}

		p := r.createProvider(providerName, apiKey, model)
		if p != nil {
			r.logger.Info("using BYOK key",
				"provider", providerName,
				"source", source,
				"company_id", companyID,
				"hint", byokKey.KeyHint,
			)
			return ResolvedProvider{Provider: p, IsBYOK: true, Source: source}
		}
	}

	return ResolvedProvider{Provider: r.defaultProvider, Source: "platform"}
}

// ResolveForProvider finds a BYOK key for a specific provider, or falls back to platform key.
func (r *Resolver) ResolveForProvider(ctx context.Context, companyID, userID int64, providerName string) ResolvedProvider {
	if r.encryptor != nil {
		byokKey, err := r.queries.ResolveByokKey(ctx, store.ResolveByokKeyParams{
			CompanyID: companyID,
			Provider:  providerName,
			UserID:    &userID,
		})
		if err == nil {
			plaintext, err := r.encryptor.Decrypt(byokKey.EncryptedKey)
			if err == nil {
				source := "company_byok"
				if byokKey.UserID != nil {
					source = "user_byok"
				}
				p := r.createProvider(providerName, string(plaintext), byokKey.ModelOverride)
				if p != nil {
					return ResolvedProvider{Provider: p, IsBYOK: true, Source: source}
				}
			}
		}
	}

	// Fallback to platform default for this provider
	p := r.createProvider(providerName, r.platformKey(providerName), "")
	if p != nil {
		return ResolvedProvider{Provider: p, Source: "platform"}
	}

	// Last resort: use the default provider regardless of providerName
	return ResolvedProvider{Provider: r.defaultProvider, Source: "platform"}
}

func (r *Resolver) createProvider(name, apiKey, model string) provider.Provider {
	if apiKey == "" {
		return nil
	}
	switch name {
	case "anthropic":
		return provider.NewAnthropic(apiKey, model)
	case "openai":
		return provider.NewOpenAI(apiKey, model)
	default:
		return nil
	}
}

func (r *Resolver) platformKey(providerName string) string {
	switch providerName {
	case "anthropic":
		return r.defaultAnthropic
	case "openai":
		return r.defaultOpenAI
	case "gemini":
		return r.defaultGemini
	default:
		return ""
	}
}

// MakeKeyHint creates a masked hint from an API key, e.g. "sk-...ab3F"
func MakeKeyHint(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	prefix := key[:3]
	suffix := key[len(key)-4:]
	return fmt.Sprintf("%s...%s", prefix, suffix)
}
