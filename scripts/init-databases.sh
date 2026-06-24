#!/bin/bash
# Initialize multiple PostgreSQL databases for flashbacks microservices

set -e

# Create additional databases beyond the default POSTGRES_DB
if [ -n "$POSTGRES_MULTIPLE_DATABASES" ]; then
    echo "Creating multiple databases: $POSTGRES_MULTIPLE_DATABASES"
    
    for db in $(echo $POSTGRES_MULTIPLE_DATABASES | tr ',' ' '); do
        if [ "$db" != "$POSTGRES_DB" ]; then
            echo "Creating database: $db"
            psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
                CREATE DATABASE "$db";
                GRANT ALL PRIVILEGES ON DATABASE "$db" TO "$POSTGRES_USER";
EOSQL
        fi
    done
    
    echo "Multiple databases created successfully."
fi
