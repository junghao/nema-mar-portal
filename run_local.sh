#!/bin/bash
#
# Local development script for nema-mar-portal.
#
# Starts PostgreSQL + FastSchema in Docker, then runs nema-mar-app.
#
# Prerequisites: docker, go
#
# Usage:
#   ./run_local.sh          # start everything
#   ./run_local.sh stop     # tear down containers
#
# Override defaults via environment:
#   DB_PORT=5433 FS_PORT=8001 APP_PORT=8081 ./run_local.sh

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONTAINER_PREFIX="nema-mar-local"

# --- Load env.list if present (env vars on CLI still take precedence) ---
ENV_FILE="${PROJECT_DIR}/env.list"
if [ -f "${ENV_FILE}" ]; then
    while IFS='=' read -r key value; do
        key="$(echo "${key}" | xargs)"
        [ -z "${key}" ] || [ "${key:0:1}" = "#" ] && continue
        value="$(echo "${value}" | xargs)"
        # Only set if not already in environment
        if [ -z "${!key+x}" ]; then
            export "${key}=${value}"
        fi
    done < "${ENV_FILE}"
fi

# --- Apply defaults for anything still unset ---
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-fastschema}"
DB_USER="${DB_USER:-fastschema}"
DB_PASS="${DB_PASS:-fastschema}"

FS_PORT="${FS_PORT:-8000}"
FS_APP_KEY="${FS_APP_KEY:-local_dev_key_32_characters_ok_}"

STORAGE_BUCKET="${STORAGE_BUCKET:-}"
STORAGE_REGION="${STORAGE_REGION:-ap-southeast-2}"

APP_PORT="${APP_PORT:-8080}"

FS_ADMIN_USER="${FS_ADMIN_USER:-admin}"
FS_ADMIN_PASS="${FS_ADMIN_PASS:-admin123}"
FASTSCHEMA_URL="${FASTSCHEMA_URL:-http://localhost:${FS_PORT}}"

SMTP_HOST="${SMTP_HOST:-}"
SMTP_PORT="${SMTP_PORT:-587}"
SMTP_USERNAME="${SMTP_USERNAME:-}"
SMTP_PASSWORD="${SMTP_PASSWORD:-}"
SMTP_FROM="${SMTP_FROM:-}"
SMTP_RECIPIENTS="${SMTP_RECIPIENTS:-}"
DDOG_API_KEY="${DDOG_API_KEY:-}"

# ---- Stop command ----
if [ "${1:-}" = "stop" ]; then
    echo "Stopping containers..."
    docker rm -f "${CONTAINER_PREFIX}-fastschema" "${CONTAINER_PREFIX}-postgres" 2>/dev/null || true
    echo "Done."
    exit 0
fi

# ---- Build FastSchema storage env ----
# STORAGE must be a JSON object: {"default_disk":"...","disks":[...]}
FS_STORAGE_ARGS=()
if [ -n "${STORAGE_BUCKET}" ]; then
    STORAGE_JSON="{\"default_disk\":\"s3\",\"disks\":[{\"name\":\"s3\",\"driver\":\"s3\",\"root\":\"nema-mar/files\",\"bucket\":\"${STORAGE_BUCKET}\",\"region\":\"${STORAGE_REGION}\",\"base_url\":\"https://${STORAGE_BUCKET}.s3.${STORAGE_REGION}.amazonaws.com\",\"public_path\":\"/files\"}]}"
    FS_STORAGE_ARGS=(-e "STORAGE=${STORAGE_JSON}")
fi

# ---- Start FastSchema ----
if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_PREFIX}-fastschema$"; then
    echo "FastSchema already running."
else
    echo "Starting FastSchema..."
    docker rm -f "${CONTAINER_PREFIX}-fastschema" 2>/dev/null || true
    docker run -d \
        --name "${CONTAINER_PREFIX}-fastschema" \
        -e APP_KEY="${FS_APP_KEY}" \
        -e APP_PORT="${FS_PORT}" \
        -e DB_DRIVER=pgx \
        -e DB_HOST=host.docker.internal \
        -e DB_PORT="${DB_PORT}" \
        -e DB_NAME="${DB_NAME}" \
        -e DB_USER="${DB_USER}" \
        -e DB_PASS="${DB_PASS}" \
        "${FS_STORAGE_ARGS[@]}" \
        -p "${FS_PORT}:${FS_PORT}" \
        ghcr.io/fastschema/fastschema:latest

    echo "Waiting for FastSchema at http://localhost:${FS_PORT}..."
    for i in $(seq 1 30); do
        if curl -sf "http://localhost:${FS_PORT}/api/health" >/dev/null 2>&1; then
            echo "FastSchema ready."
            break
        fi
        if [ "$i" -eq 30 ]; then
            echo "WARNING: FastSchema did not become ready in time."
            echo "Check logs: docker logs ${CONTAINER_PREFIX}-fastschema"
        fi
        sleep 2
    done
fi

# ---- Print config ----
cat <<EOF

=== Local Development Environment ===

  PostgreSQL:  localhost:${DB_PORT}  (${DB_USER}/${DB_PASS}@${DB_NAME})
  FastSchema:  http://localhost:${FS_PORT}
  App:         http://localhost:${APP_PORT}

  Dashboard:   http://localhost:${APP_PORT}/dashboard
  EAT Editor:  http://localhost:${APP_PORT}/gha-portal
  Health:      http://localhost:${APP_PORT}/soh

  FastSchema setup UI (first run only):
    Check: docker logs ${CONTAINER_PREFIX}-fastschema | grep token

  Stop everything:  ./run_local.sh stop

=== Starting nema-mar-app ===

EOF

# ---- Run the Go app ----
export FASTSCHEMA_URL="${FASTSCHEMA_URL}"
export FS_ADMIN_USER="${FS_ADMIN_USER}"
export FS_ADMIN_PASS="${FS_ADMIN_PASS}"
export SMTP_HOST="${SMTP_HOST}"
export SMTP_PORT="${SMTP_PORT}"
export SMTP_USERNAME="${SMTP_USERNAME}"
export SMTP_PASSWORD="${SMTP_PASSWORD}"
export SMTP_FROM="${SMTP_FROM}"
export SMTP_RECIPIENTS="${SMTP_RECIPIENTS}"
export DDOG_API_KEY="${DDOG_API_KEY}"

cd "${PROJECT_DIR}"
exec go run ./cmd/nema-mar-app
