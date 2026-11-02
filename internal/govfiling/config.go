package govfiling

import (
	"log/slog"
	"os"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/integration/crypto"
)

// ServiceConfigFromEnv builds the agent config shared by the API and worker.
//
//	GOV_FILES_DIR        private directory for returns & evidence (default ./data/gov)
//	CHROME_PATH          Chromium binary for the browser channel
//	GOV_MOCK_PORTAL_URL  enables the demo mock portal form
//	GOV_EMAIL_OVERRIDE_TO staging only: redirect e-mail submissions
func ServiceConfigFromEnv(encKey string, ai provider.Provider, mail *email.Sender, logger *slog.Logger) Config {
	cfg := Config{FileDir: os.Getenv("GOV_FILES_DIR"), Mail: mail}
	if encKey != "" {
		if e, err := crypto.NewCredentialEncryptor(encKey); err == nil {
			cfg.Encryptor = e
		} else {
			logger.Error("govfiling: invalid INTEGRATION_ENCRYPTION_KEY", "error", err)
		}
	}
	if ai != nil {
		cfg.AI = &AIReviewer{Provider: ai}
	}
	return cfg
}
