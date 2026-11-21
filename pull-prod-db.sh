#!/bin/bash -ex
set -o pipefail

SSH_HOST="themylogin@192.168.0.1"
REMOTE_DIR="/home/themylogin/apps/ownocr"
REMOTE_CONTAINER="ownocr-postgres-1"
REMOTE_DB_USER="ownocr"
REMOTE_DB="ownocr"

LOCAL_CONTAINER="ownocr-dev-postgres-1"
LOCAL_BACKEND="ownocr-dev-backend-1"
LOCAL_DB_USER="ownocr"
LOCAL_DB="ownocr"

DUMP_FILE="ownocr.psql"

ssh "$SSH_HOST" "cd $REMOTE_DIR && sudo docker exec -i $REMOTE_CONTAINER pg_dump -U $REMOTE_DB_USER $REMOTE_DB" > "$DUMP_FILE"

sudo docker stop "$LOCAL_BACKEND"

echo "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$LOCAL_DB' AND pid <> pg_backend_pid();" | sudo docker exec -i "$LOCAL_CONTAINER" psql -U "$LOCAL_DB_USER" -d postgres
sudo docker exec -i "$LOCAL_CONTAINER" dropdb -U "$LOCAL_DB_USER" "$LOCAL_DB"
sudo docker exec -i "$LOCAL_CONTAINER" createdb -U "$LOCAL_DB_USER" "$LOCAL_DB"
cat "$DUMP_FILE" | sudo docker exec -i "$LOCAL_CONTAINER" psql -U "$LOCAL_DB_USER" -d "$LOCAL_DB"

sudo docker start "$LOCAL_BACKEND"
