# AWS Secrets Manager for credentials

Sensitive values (API keys, private keys, builder credentials) can live in a **single** AWS Secrets Manager secret as a JSON object. Go binaries and Python scripts load that JSON into the process environment when `POLYMARKET_SECRETS_MANAGER_SECRET_ID` is set.

Non-secret configuration (CLOB base URL, log level, data API host, etc.) stays in `.env` or the shell as today.

## How loading works

1. `godotenv.Load()` (Go) or `load_dotenv()` (Python) runs first.
2. If `POLYMARKET_SECRETS_MANAGER_SECRET_ID` is non-empty, the app fetches the secret once, parses JSON, and sets **only keys present with non-empty string values** via `os.Setenv` / `os.environ` (overwriting any value already set for those keys).
3. Existing code (`clob.NewClient`, `placeorder`, etc.) still reads from the environment; no change to those call sites.

Entry points that call the loader:

| Command | Loader |
|--------|--------|
| `go run .` / `make run` | `secrets.MustLoadFromEnvIfConfigured` in `main.go` |
| `make live-clob` | same in `cmd/liveclob/main.go` |
| `make place-order` | same in `cmd/placeorder/main.go` |
| `make creds` | `must_load_from_env_if_configured()` in `cmd/python/generate_creds.py` |

If the secret ID is set and the fetch fails, the process logs an error and exits (misconfiguration should be obvious).

## Secret JSON shape

Store a **string** secret whose value is JSON: an object mapping env var names to string values. Include only keys your deployment needs.

```json
{
  "POLYMARKET_API_KEY": "...",
  "POLYMARKET_API_SECRET": "...",
  "POLYMARKET_PASSPHRASE": "...",
  "POLYMARKET_ADDRESS": "0x...",
  "POLYMARKET_DEPOSIT_WALLET": "0x...",
  "POLYMARKET_PRIVATE_KEY": "0x...",
  "METAMASK_KEY": "0x...",
  "BUILDER_API_KEY": "...",
  "BUILDER_SECRET": "...",
  "BUILDER_PASSPHRASE": "..."
}
```

Do not commit real values. For local dev you can keep a `secret.json` (gitignored) and pass it to the AWS CLI with `file://`.

## Configuration (non-secret)

| Variable | Purpose |
|----------|---------|
| `POLYMARKET_SECRETS_MANAGER_SECRET_ID` | **Required to enable SM.** Secret name or full ARN. |
| `AWS_ENDPOINT_URL` | **LocalStack only.** e.g. `http://localhost:4566`. Unset for real AWS. |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | Dummy values for LocalStack; real credentials or omitted when using IAM roles on AWS. |
| `AWS_REGION` or `AWS_DEFAULT_REGION` | Region where the secret lives (e.g. `us-east-1`). |

Go uses the AWS SDK v2 default config (`config.LoadDefaultConfig`), which honors `AWS_ENDPOINT_URL` for custom endpoints. Python uses `boto3` with `endpoint_url` only when `AWS_ENDPOINT_URL` is set.

Other app env vars (`POLYMARKET_CLOB_BASE_URL`, `LOG_LEVEL`, `POLYMARKET_DATA_API_BASE_URL`, …) are unchanged and are **not** read from Secrets Manager unless you add them to the JSON (not recommended for non-secrets).

---

## LocalStack (development)

### Docker Compose (recommended)

[`docker-compose.yml`](../docker-compose.yml) runs **LocalStack** (Secrets Manager on port **4566**) and the **polymarket** app on a shared network. Inside the app container, `AWS_ENDPOINT_URL` is `http://localstack:4566`; on the host use `http://localhost:4566`.

```bash
make up          # docker compose up -d --build
make down        # tear down LocalStack + app
```

Before the app can start with Secrets Manager enabled, seed the secret (once per machine, or after `make down -v`):

```bash
# copy docs example into gitignored secret.json, fill in real dev values
make localstack-init-secret
```

The compose file sets `POLYMARKET_SECRETS_MANAGER_SECRET_ID=polymarket/dev` and dummy AWS credentials for the app service. Non-secret settings can still come from `.env` via `env_file`.

For **host-run** commands (`make run`, `make live-clob`, `go run`, `make creds`), export `AWS_ENDPOINT_URL=http://localhost:4566` and the same secret id (see below).

### Run LocalStack only (manual)

If you prefer not to use Compose:

```bash
docker run -d --name localstack \
  -p 4566:4566 \
  -e SERVICES=secretsmanager \
  localstack/localstack
```

Check health: `curl -s http://localhost:4566/_localstack/health` (or wait until the container is ready).

### Shell environment for LocalStack

Export these before `make run`, `make live-clob`, `make place-order`, or `make creds`:

```bash
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
export POLYMARKET_SECRETS_MANAGER_SECRET_ID=polymarket/dev
```

`POLYMARKET_SECRETS_MANAGER_SECRET_ID` is the trigger: without it, nothing is fetched from Secrets Manager.

### Create or update the secret (AWS CLI v2)

Create a JSON file locally (example `secret.json`, **do not commit**):

```json
{
  "POLYMARKET_API_KEY": "your-key",
  "POLYMARKET_API_SECRET": "your-secret",
  "POLYMARKET_PASSPHRASE": "your-passphrase",
  "POLYMARKET_ADDRESS": "0xYourEoaAddress"
}
```

Create the secret once:

```bash
AWS_ENDPOINT_URL=http://localhost:4566 \
  aws secretsmanager create-secret \
  --name polymarket/dev \
  --secret-string file://secret.json
```

Update an existing secret:

```bash
AWS_ENDPOINT_URL=http://localhost:4566 \
  aws secretsmanager put-secret-value \
  --secret-id polymarket/dev \
  --secret-string file://secret.json
```

Use the same `--name` / `secret-id` as `POLYMARKET_SECRETS_MANAGER_SECRET_ID`.

### Python dependencies

From the repo root:

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r cmd/python/requirements.txt
```

### Example workflow

```bash
# Terminal 1: LocalStack (see above)

# Terminal 2: app
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
export POLYMARKET_SECRETS_MANAGER_SECRET_ID=polymarket/dev

make live-clob
# or: go run . ; make place-order ; make creds
```

Keep non-sensitive settings in `.env` (e.g. `POLYMARKET_CLOB_BASE_URL`).

---

## Switching to real AWS

Application code is the same; only environment, credentials, and IAM change.

### Endpoint and credentials

1. **Unset** `AWS_ENDPOINT_URL` so the SDK and boto3 use the public AWS endpoints for your region.
2. **Credentials**
   - **Local dev:** set real `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` (or use `aws configure` / SSO profiles supported by the default credential chain).
   - **EC2 / ECS / Lambda / EKS:** rely on the instance/task/pod **IAM role**; do not embed long-lived keys in the environment.
3. **Region:** set `AWS_REGION` or `AWS_DEFAULT_REGION` to the region where you created the secret.
4. **Secret id:** set `POLYMARKET_SECRETS_MANAGER_SECRET_ID` to the secret **name** or full **ARN** in that account/region.

### IAM policy

Grant `secretsmanager:GetSecretValue` on the secret (least privilege):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "arn:aws:secretsmanager:us-east-1:ACCOUNT_ID:secret:polymarket/dev-*"
    }
  ]
}
```

Attach the policy to the IAM user/role used by your laptop, CI job, or runtime service.

### Optional hardening

- **VPC interface endpoint** for Secrets Manager in private subnets so traffic does not leave your VPC.
- **Secret rotation:** this loader always reads the current `SecretString` from `GetSecretValue` (AWS returns the `AWSCURRENT` version). Automatic rotation with staged versions is not specially handled; if you enable rotation, ensure clients tolerate brief overlap or document a maintenance window.

### Tests

Go unit tests in `secrets/` exercise JSON parsing and env application without network. Package tests elsewhere do not set `POLYMARKET_SECRETS_MANAGER_SECRET_ID`, so they keep using `t.Setenv` only.
