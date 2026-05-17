#!/bin/bash
set -euo pipefail
exec > >(tee /var/log/user-data.log) 2>&1

# Amazon Linux 2023 includes AWS CLI v2; install if the AMI variant lacks it.
if ! command -v aws >/dev/null 2>&1; then
  dnf install -y aws-cli
fi

aws s3 cp "s3://${artifact_bucket}/${artifact_key}" /usr/local/bin/polymarket --region "${aws_region}"
chmod +x /usr/local/bin/polymarket

touch /etc/polymarket.env
chmod 600 /etc/polymarket.env

cat >/etc/systemd/system/polymarket.service <<'UNIT'
[Unit]
Description=Polymarket application (${project_name})
Documentation=file:///etc/polymarket.env
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=AWS_REGION=${aws_region}
Environment=POLYMARKET_SECRETS_MANAGER_SECRET_ID=${secrets_manager_secret_name}
EnvironmentFile=-/etc/polymarket.env
ExecStart=/usr/local/bin/polymarket
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable polymarket.service
