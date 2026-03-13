#!/bin/bash
set -e

# psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "CREATE ROLE postgres WITH LOGIN SUPERUSER PASSWORD '$POSTGRES_PASSWORD';"
echo "cron.database_name = '${POSTGRES_DB:-firecrawl}'" >> "$PGDATA/postgresql.conf"

# 1. Setup Firecrawl (Using its own owner)
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE USER ${FIRE_POSTGRES_USER} WITH PASSWORD '${FIRE_POSTGRES_PASSWORD}';
    CREATE DATABASE ${FIRE_POSTGRES_DB} OWNER ${FIRE_POSTGRES_USER};
EOSQL

# Run the schema as ROOT (to handle extensions/schema creation)
if [ -f "/firecrawl_setup/nuq.sql" ]; then
    echo "Applying Firecrawl schema to ${FIRE_POSTGRES_DB}..."
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${FIRE_POSTGRES_DB}" -f /firecrawl_setup/nuq.sql
    
    # DIAGNOSIS REPAIR: Hand over the nuq schema to the service user
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${FIRE_POSTGRES_DB}" <<-EOSQL
        GRANT USAGE ON SCHEMA nuq TO ${FIRE_POSTGRES_USER};
        GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA nuq TO ${FIRE_POSTGRES_USER};
        GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA nuq TO ${FIRE_POSTGRES_USER};
        -- Also grant public schema rights just in case they have legacy tables
        GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${FIRE_POSTGRES_USER};
EOSQL
fi

# 2. Setup LangGraph (Using its own owner)
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE USER ${LANG_POSTGRES_USER} WITH PASSWORD '${LANG_POSTGRES_PASSWORD}';
    CREATE DATABASE langgraph_db OWNER ${LANG_POSTGRES_USER};
    -- LangGraph checkpointer will create its own tables under this owner
EOSQL

# 3. The "Lockdown": Revoke public access so they can't peek at each other
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    REVOKE ALL ON DATABASE ${FIRE_POSTGRES_DB} FROM PUBLIC;
    REVOKE ALL ON DATABASE ${LANG_POSTGRES_DB} FROM PUBLIC;
EOSQL