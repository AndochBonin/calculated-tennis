package secrets

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

const secretIDEnv = "POLYMARKET_SECRETS_MANAGER_SECRET_ID"

var processExit = os.Exit

// LoadFromEnvIfConfigured loads credentials from AWS Secrets Manager when
// POLYMARKET_SECRETS_MANAGER_SECRET_ID is set. Call after godotenv.Load().
func LoadFromEnvIfConfigured(ctx context.Context) error {
	secretID := strings.TrimSpace(os.Getenv(secretIDEnv))
	if secretID == "" {
		return nil
	}
	return LoadFromSecretsManager(ctx, secretID)
}

// MustLoadFromEnvIfConfigured is like LoadFromEnvIfConfigured but logs and exits on failure.
func MustLoadFromEnvIfConfigured(ctx context.Context, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	if err := LoadFromEnvIfConfigured(ctx); err != nil {
		log.Error("failed to load secrets from AWS Secrets Manager", "err", err, "secret_id_env", secretIDEnv)
		processExit(1)
	}
}
