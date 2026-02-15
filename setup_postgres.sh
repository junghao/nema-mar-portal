#!/bin/bash
#
# Initialize a local PostgreSQL instance for FastSchema.
#
# This creates the database and role that FastSchema expects.
# Run this once before running ./run_local.sh.
#
# Prerequisites:
#   - PostgreSQL running locally (e.g. via Homebrew: brew services start postgresql@16)
#   - psql available on PATH
#
# Usage:
#   ./setup_postgres.sh          # create database and role
#   ./setup_postgres.sh teardown # drop database and role
#
# Reads configuration from env.list if present, or uses defaults.

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"

# --- Load env.list if present (env vars on CLI still take precedence) ---
ENV_FILE="${PROJECT_DIR}/env.list"
if [ -f "${ENV_FILE}" ]; then
    while IFS='=' read -r key value; do
        key="$(echo "${key}" | xargs)"
        [ -z "${key}" ] || [ "${key:0:1}" = "#" ] && continue
        value="$(echo "${value}" | xargs)"
        if [ -z "${!key+x}" ]; then
            export "${key}=${value}"
        fi
    done < "${ENV_FILE}"
fi

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-fastschema}"
DB_USER="${DB_USER:-fastschema}"
DB_PASS="${DB_PASS:-fastschema}"

# Use PGHOST/PGPORT so psql connects to the right place
export PGHOST="${DB_HOST}"
export PGPORT="${DB_PORT}"

if [ "${1:-}" = "teardown" ]; then
    echo "Dropping database '${DB_NAME}' and role '${DB_USER}'..."
    psql -U postgres -c "DROP DATABASE IF EXISTS ${DB_NAME};"
    psql -U postgres -c "DROP ROLE IF EXISTS ${DB_USER};"
    echo "Done."
    exit 0
fi

echo "Setting up PostgreSQL for FastSchema..."
echo "  Host: ${DB_HOST}:${DB_PORT}"
echo "  Database: ${DB_NAME}"
echo "  User: ${DB_USER}"
echo ""

# Create role if it doesn't exist
if psql -U postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1; then
    echo "Role '${DB_USER}' already exists."
else
    echo "Creating role '${DB_USER}'..."
    psql -U postgres -c "CREATE ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASS}';"
fi

# Create database if it doesn't exist
if psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
    echo "Database '${DB_NAME}' already exists."
else
    echo "Creating database '${DB_NAME}'..."
    psql -U postgres -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"
fi

# Grant privileges
psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"

echo ""
echo "PostgreSQL is ready for FastSchema."
echo "  Connection: postgresql://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo ""
echo "Next: ./run_local.sh"
