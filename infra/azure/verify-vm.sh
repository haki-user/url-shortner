#!/usr/bin/env sh
set -eu

key_vault_name="${1:?key vault name is required}"

cloud-init status --wait
docker --version
docker compose version
az version

az login --identity --output none
az keyvault secret show \
	--vault-name "$key_vault_name" \
	--name tinyurl-database-url \
	--query name \
	--output tsv
