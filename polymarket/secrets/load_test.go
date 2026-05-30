package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeSecretsManager struct {
	out      *secretsmanager.GetSecretValueOutput
	err      error
	secretID string
}

func (f *fakeSecretsManager) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if in != nil && in.SecretId != nil {
		f.secretID = *in.SecretId
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestApplySecretMap(t *testing.T) {
	t.Setenv("POLYMARKET_API_KEY", "from-test")
	t.Setenv("POLYMARKET_API_SECRET", "from-test")

	m := map[string]string{
		"POLYMARKET_API_KEY":    "from-secret",
		"POLYMARKET_PASSPHRASE": "pass",
		"":                      "ignored",
		"EMPTY_VALUE":           "",
		"  ":                    "trim-key-skipped",
	}

	if err := applySecretMap(m); err != nil {
		t.Fatalf("applySecretMap: %v", err)
	}

	if got := os.Getenv("POLYMARKET_API_KEY"); got != "from-secret" {
		t.Errorf("POLYMARKET_API_KEY = %q, want from-secret", got)
	}
	if got := os.Getenv("POLYMARKET_PASSPHRASE"); got != "pass" {
		t.Errorf("POLYMARKET_PASSPHRASE = %q, want pass", got)
	}
	if got := os.Getenv("POLYMARKET_API_SECRET"); got != "from-test" {
		t.Errorf("POLYMARKET_API_SECRET = %q, want unchanged from-test", got)
	}
	if got := os.Getenv("EMPTY_VALUE"); got != "" {
		t.Errorf("EMPTY_VALUE = %q, want empty (not set)", got)
	}
	if got := os.Getenv("  "); got != "" {
		t.Errorf("whitespace key env = %q, want unset", got)
	}
}

func TestApplySecretMap_skipsWhitespaceValues(t *testing.T) {
	m := map[string]string{
		"POLYMARKET_API_KEY": "  ",
		"POLYMARKET_ADDRESS": "0x1",
	}
	if err := applySecretMap(m); err != nil {
		t.Fatalf("applySecretMap: %v", err)
	}
	if got := os.Getenv("POLYMARKET_API_KEY"); got != "" {
		t.Errorf("POLYMARKET_API_KEY = %q, want unset (whitespace-only value)", got)
	}
	if got := os.Getenv("POLYMARKET_ADDRESS"); got != "0x1" {
		t.Errorf("POLYMARKET_ADDRESS = %q, want 0x1", got)
	}
}

func TestLoadFromClient(t *testing.T) {
	secretJSON, err := json.Marshal(map[string]string{
		"POLYMARKET_ADDRESS": "0xabc",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(secretJSON)

	t.Run("success", func(t *testing.T) {
		t.Setenv("POLYMARKET_ADDRESS", "")
		client := &fakeSecretsManager{
			out: &secretsmanager.GetSecretValueOutput{SecretString: &s},
		}
		if err := loadFromClient(context.Background(), client, "polymarket/dev"); err != nil {
			t.Fatalf("loadFromClient: %v", err)
		}
		if client.secretID != "polymarket/dev" {
			t.Errorf("GetSecretValue SecretId = %q, want polymarket/dev", client.secretID)
		}
		if got := os.Getenv("POLYMARKET_ADDRESS"); got != "0xabc" {
			t.Errorf("POLYMARKET_ADDRESS = %q, want 0xabc", got)
		}
	})

	t.Run("multiple keys", func(t *testing.T) {
		multi, err := json.Marshal(map[string]string{
			"POLYMARKET_API_KEY":    "key",
			"POLYMARKET_API_SECRET": "secret",
		})
		if err != nil {
			t.Fatal(err)
		}
		multiStr := string(multi)
		client := &fakeSecretsManager{
			out: &secretsmanager.GetSecretValueOutput{SecretString: &multiStr},
		}
		if err := loadFromClient(context.Background(), client, "arn:aws:secretsmanager:us-east-1:123:secret:x"); err != nil {
			t.Fatalf("loadFromClient: %v", err)
		}
		if got := os.Getenv("POLYMARKET_API_KEY"); got != "key" {
			t.Errorf("POLYMARKET_API_KEY = %q, want key", got)
		}
		if got := os.Getenv("POLYMARKET_API_SECRET"); got != "secret" {
			t.Errorf("POLYMARKET_API_SECRET = %q, want secret", got)
		}
	})

	t.Run("get error", func(t *testing.T) {
		client := &fakeSecretsManager{err: errors.New("network down")}
		err := loadFromClient(context.Background(), client, "polymarket/dev")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no secret string", func(t *testing.T) {
		client := &fakeSecretsManager{out: &secretsmanager.GetSecretValueOutput{}}
		err := loadFromClient(context.Background(), client, "polymarket/dev")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		bad := "not-json"
		client := &fakeSecretsManager{
			out: &secretsmanager.GetSecretValueOutput{SecretString: &bad},
		}
		err := loadFromClient(context.Background(), client, "polymarket/dev")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDefaultSecretsManagerClient(t *testing.T) {
	if defaultSecretsManagerClient(aws.Config{}) == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestLoadFromSecretsManager_emptyID(t *testing.T) {
	err := LoadFromSecretsManager(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty secret ID")
	}
}

func withSecretsHooks(t *testing.T, cfg aws.Config, cfgErr error, client secretsManagerAPI) {
	t.Helper()
	oldLoad := loadAWSConfig
	oldNew := newSecretsManagerFromConfig
	t.Cleanup(func() {
		loadAWSConfig = oldLoad
		newSecretsManagerFromConfig = oldNew
	})
	loadAWSConfig = func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
		return cfg, cfgErr
	}
	newSecretsManagerFromConfig = func(aws.Config) secretsManagerAPI {
		return client
	}
}

func TestLoadFromSecretsManager(t *testing.T) {
	t.Run("success trims secret ID", func(t *testing.T) {
		secretJSON := `{"POLYMARKET_API_KEY":"from-sm"}`
		client := &fakeSecretsManager{
			out: &secretsmanager.GetSecretValueOutput{SecretString: &secretJSON},
		}
		withSecretsHooks(t, aws.Config{}, nil, client)

		t.Setenv("POLYMARKET_API_KEY", "")
		if err := LoadFromSecretsManager(context.Background(), "  polymarket/dev  "); err != nil {
			t.Fatalf("LoadFromSecretsManager: %v", err)
		}
		if client.secretID != "polymarket/dev" {
			t.Errorf("SecretId = %q, want polymarket/dev", client.secretID)
		}
		if got := os.Getenv("POLYMARKET_API_KEY"); got != "from-sm" {
			t.Errorf("POLYMARKET_API_KEY = %q, want from-sm", got)
		}
	})

	t.Run("config error", func(t *testing.T) {
		withSecretsHooks(t, aws.Config{}, errors.New("no credentials"), nil)
		err := LoadFromSecretsManager(context.Background(), "polymarket/dev")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestApplySecretMap_setenvError(t *testing.T) {
	old := envSet
	t.Cleanup(func() { envSet = old })
	envSet = func(k, v string) error {
		if k == "POLYMARKET_FAIL" {
			return errors.New("setenv failed")
		}
		return os.Setenv(k, v)
	}
	err := applySecretMap(map[string]string{"POLYMARKET_FAIL": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFromEnvIfConfigured(t *testing.T) {
	t.Run("skips when secret ID env unset", func(t *testing.T) {
		t.Setenv(secretIDEnv, "")
		if err := LoadFromEnvIfConfigured(context.Background()); err != nil {
			t.Fatalf("LoadFromEnvIfConfigured: %v", err)
		}
	})

	t.Run("skips when secret ID env whitespace only", func(t *testing.T) {
		t.Setenv(secretIDEnv, "  \t  ")
		if err := LoadFromEnvIfConfigured(context.Background()); err != nil {
			t.Fatalf("LoadFromEnvIfConfigured: %v", err)
		}
	})

	t.Run("loads when secret ID env set", func(t *testing.T) {
		secretJSON := `{"POLYMARKET_ADDRESS":"0xenv"}`
		client := &fakeSecretsManager{
			out: &secretsmanager.GetSecretValueOutput{SecretString: &secretJSON},
		}
		withSecretsHooks(t, aws.Config{}, nil, client)

		t.Setenv(secretIDEnv, "polymarket/dev")
		if err := LoadFromEnvIfConfigured(context.Background()); err != nil {
			t.Fatalf("LoadFromEnvIfConfigured: %v", err)
		}
		if got := os.Getenv("POLYMARKET_ADDRESS"); got != "0xenv" {
			t.Errorf("POLYMARKET_ADDRESS = %q, want 0xenv", got)
		}
	})
}

func TestMustLoadFromEnvIfConfigured(t *testing.T) {
	t.Run("no-op when secret ID unset", func(t *testing.T) {
		t.Setenv(secretIDEnv, "")
		MustLoadFromEnvIfConfigured(context.Background(), slog.Default())
	})

	t.Run("nil logger uses default", func(t *testing.T) {
		t.Setenv(secretIDEnv, "")
		MustLoadFromEnvIfConfigured(context.Background(), nil)
	})

	t.Run("success when configured", func(t *testing.T) {
		secretJSON := `{"POLYMARKET_API_KEY":"ok"}`
		client := &fakeSecretsManager{
			out: &secretsmanager.GetSecretValueOutput{SecretString: &secretJSON},
		}
		withSecretsHooks(t, aws.Config{}, nil, client)
		t.Setenv(secretIDEnv, "polymarket/dev")
		MustLoadFromEnvIfConfigured(context.Background(), slog.Default())
	})

	t.Run("exits on load error", func(t *testing.T) {
		withSecretsHooks(t, aws.Config{}, errors.New("cfg"), nil)
		t.Setenv(secretIDEnv, "polymarket/dev")

		exited := false
		var exitCode int
		oldExit := processExit
		processExit = func(code int) {
			exited = true
			exitCode = code
		}
		t.Cleanup(func() { processExit = oldExit })

		var logBuf bytes.Buffer
		log := slog.New(slog.NewTextHandler(&logBuf, nil))
		MustLoadFromEnvIfConfigured(context.Background(), log)

		if !exited {
			t.Fatal("expected processExit to be called")
		}
		if exitCode != 1 {
			t.Errorf("exit code = %d, want 1", exitCode)
		}
		if !bytes.Contains(logBuf.Bytes(), []byte("failed to load secrets")) {
			t.Errorf("log = %q, want error message", logBuf.String())
		}
	})
}
