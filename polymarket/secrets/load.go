package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func defaultSecretsManagerClient(cfg aws.Config) secretsManagerAPI {
	return secretsmanager.NewFromConfig(cfg)
}

// Test hooks (overridden in tests).
var (
	loadAWSConfig               = config.LoadDefaultConfig
	newSecretsManagerFromConfig = defaultSecretsManagerClient
	envSet                      = os.Setenv
)

// secretsManagerAPI is the subset of secretsmanager.Client used by the loader (for tests).
type secretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// LoadFromSecretsManager fetches secretID from AWS Secrets Manager, parses the value as
// JSON object of string keys to string values, and sets each non-empty key in the process
// environment via os.Setenv.
func LoadFromSecretsManager(ctx context.Context, secretID string) error {
	secretID = strings.TrimSpace(secretID)
	if secretID == "" {
		return fmt.Errorf("secrets: empty secret ID")
	}

	cfg, err := loadAWSConfig(ctx)
	if err != nil {
		return fmt.Errorf("secrets: load AWS config: %w", err)
	}

	client := newSecretsManagerFromConfig(cfg)
	return loadFromClient(ctx, client, secretID)
}

func loadFromClient(ctx context.Context, client secretsManagerAPI, secretID string) error {
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretID,
	})
	if err != nil {
		return fmt.Errorf("secrets: get secret value: %w", err)
	}

	if out.SecretString == nil {
		return fmt.Errorf("secrets: secret %q has no SecretString", secretID)
	}

	var m map[string]string
	if err := json.Unmarshal([]byte(*out.SecretString), &m); err != nil {
		return fmt.Errorf("secrets: parse secret JSON: %w", err)
	}

	return applySecretMap(m)
}

// applySecretMap sets os.Setenv for each key in m with a non-empty string value.
func applySecretMap(m map[string]string) error {
	for k, v := range m {
		if strings.TrimSpace(k) == "" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		if err := envSet(k, v); err != nil {
			return fmt.Errorf("secrets: set env %q: %w", k, err)
		}
	}
	return nil
}
