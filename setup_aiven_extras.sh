#!/bin/bash
# Usage:
#   ./setup_aiven_extras.sh <host> <port> <dbname> <user> <password> <sslmode>
#
# Example:
#   ./setup_aiven_extras.sh myhost.example.com 5432 mydb myuser mysecret require

if [ "$#" -ne 6 ]; then
    echo "Usage: $0 <host> <port> <dbname> <user> <password> <sslmode>"
    exit 1
fi

HOST=$1
PORT=$2
DBNAME=$3
USER=$4
PASSWORD=$5
SSLMODE=$6

# Export the password so that psql can use it without prompting
export PGPASSWORD="$PASSWORD"
CONN="host=$HOST port=$PORT dbname=$DBNAME user=$USER sslmode=$SSLMODE"

# Function to run an SQL command with error stopping enabled
run_sql() {
    SQL="$1"
    psql "$CONN" -v ON_ERROR_STOP=1 -c "$SQL"
}

echo "Installing Aiven extras extension..."
run_sql "CREATE EXTENSION IF NOT EXISTS aiven_extras CASCADE;"
if [ "$?" -ne 0 ]; then
    echo "Failed to install Aiven extras extension."
    exit 1
fi
echo "Aiven extras extension installed successfully."

echo "Modifying configuration using aiven_extras.pg_modify_settings..."
# Use the aiven_extras function to update settings without needing superuser privileges.
echo "Modifying configuration using ALTER SYSTEM..."
# Modify PostgreSQL settings
run_sql "ALTER SYSTEM SET wal_level = 'logical';"
run_sql "ALTER SYSTEM SET max_replication_slots = 10;"
run_sql "ALTER SYSTEM SET max_wal_senders = 10;"

# Reload the configuration to apply changes
run_sql "SELECT pg_reload_conf();"

if [ "$?" -eq 0 ]; then
    echo "Configuration updated successfully using ALTER SYSTEM."
else
    echo "Failed to update configuration using ALTER SYSTEM."
    exit 1
fi
# Check the current wal_level
new_wal=$(psql "$CONN" -t -c "SHOW wal_level;" | xargs)
echo "Current wal_level: $new_wal"
if [ "$new_wal" != "logical" ]; then
    echo "Note: The change to wal_level may require a full server restart to take effect."
fi
