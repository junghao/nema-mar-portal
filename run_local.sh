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
export DB_HOST="${DB_HOST:-localhost}"
export DB_PORT="${DB_PORT:-5432}"
export DB_NAME="${DB_NAME:-fastschema}"
export DB_USER="${DB_USER:-fastschema}"
export DB_PASS="${DB_PASS:-fastschema}"

export FS_PORT="${FS_PORT:-8000}"
export FS_APP_KEY="${FS_APP_KEY:-local_dev_key_32_characters_ok_}"

export STORAGE_BUCKET="${STORAGE_BUCKET:-}"
export STORAGE_REGION="${STORAGE_REGION:-ap-southeast-2}"

export APP_PORT="${APP_PORT:-8080}"

export FS_ADMIN_USER="${FS_ADMIN_USER:-admin}"
export FS_ADMIN_PASS="${FS_ADMIN_PASS:-admin123}"
export FASTSCHEMA_URL="${FASTSCHEMA_URL:-http://localhost:${FS_PORT}}"

export SMTP_HOST="${SMTP_HOST:-}"
export SMTP_PORT="${SMTP_PORT:-587}"
export SMTP_USERNAME="${SMTP_USERNAME:-}"
export SMTP_PASSWORD="${SMTP_PASSWORD:-}"
export SMTP_FROM="${SMTP_FROM:-}"
export SMTP_RECIPIENTS="${SMTP_RECIPIENTS:-}"
export DDOG_API_KEY="${DDOG_API_KEY:-}"

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

    # ---- Wait for FastSchema to be reachable ----
    echo "Waiting for FastSchema on port ${FS_PORT}..."
    READY=0
    for i in $(seq 1 30); do
        STATUS=$(curl -s -L -o /dev/null -w "%{http_code}" "http://localhost:${FS_PORT}/dash" 2>/dev/null) && true
        if [ "${STATUS}" = "200" ]; then
            READY=1
            break
        fi
        sleep 1
    done
    if [ "${READY}" -eq 0 ]; then
        echo "ERROR: FastSchema did not start in time."
        echo "Check logs: docker logs ${CONTAINER_PREFIX}-fastschema"
        exit 1
    fi

    # ---- Auto-setup admin account on first run ----
    # FastSchema emits a setup token in its logs only when no admin exists yet.
    SETUP_TOKEN=$(docker logs "${CONTAINER_PREFIX}-fastschema" 2>&1 \
        | grep -oE 'token=[A-Za-z0-9]+' | sed 's/token=//' | tail -1 || true)
    if [ -n "${SETUP_TOKEN}" ]; then
        echo "Creating admin account (user: ${FS_ADMIN_USER})..."
        SETUP_RESULT=$(curl -s -X POST "http://localhost:${FS_PORT}/api/setup" \
            -H "Content-Type: application/json" \
            -d "{\"token\":\"${SETUP_TOKEN}\",\"username\":\"${FS_ADMIN_USER}\",\"email\":\"admin@localhost\",\"password\":\"${FS_ADMIN_PASS}\"}")
        if echo "${SETUP_RESULT}" | grep -q '"data":true'; then
            echo "FastSchema admin account created."
        else
            echo "ERROR: FastSchema setup failed: ${SETUP_RESULT}"
            exit 1
        fi
    fi
    echo "FastSchema ready."
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

  Stop everything:  ./run_local.sh stop

=== Starting nema-mar-app ===

EOF

# ---- Run the Go app ----
cd "${PROJECT_DIR}"
exec go run ./cmd/nema-mar-app
