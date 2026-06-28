#!/usr/bin/env sh
set -eu

if [ "$#" -ne 4 ]; then
	echo "usage: $0 <full-image-reference> <public-url> <registry-name> <key-vault-name>" >&2
	exit 2
fi

image="$1"
public_url="$2"
registry="$3"
key_vault="$4"
deployment_dir="${TINYURL_DEPLOYMENT_DIR:-/opt/tinyurl}"

cd "$deployment_dir"

az login --identity --output none
az acr login --name "$registry" --output none

database_url="$(
	az keyvault secret show \
		--vault-name "$key_vault" \
		--name tinyurl-database-url \
		--query value \
		--output tsv
)"

cat > .env <<EOF
TINYURL_PUBLIC_URL=$public_url
TINYURL_DATABASE_URL=$database_url
AZURE_CONTAINER_REGISTRY=$registry
EOF
chmod 600 .env

printf 'TINYURL_IMAGE=%s\n' "$image" > .image.env
chmod 600 .image.env

compose="docker compose --env-file .env --env-file .image.env -f compose.yaml"

$compose pull linkd migrate
$compose --profile migration run --rm migrate
$compose up -d --remove-orphans redis linkd caddy

attempt=0
until $compose exec -T linkd wget -q -O /dev/null http://localhost:8080/readyz; do
	attempt=$((attempt + 1))
	if [ "$attempt" -ge 30 ]; then
		echo "linkd did not become ready" >&2
		$compose logs --tail 100 linkd
		exit 1
	fi
	sleep 2
done

docker image prune -f >/dev/null

echo "deployment completed: $image"
