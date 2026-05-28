#!/bin/bash
# Integration test for KOSYNC
set -e

# Config
PORT=8081
URL="http://localhost:$PORT"
DB="kosync_test.db"
USER="testuser"
PASS="testpass"
DOC="doc123"
BIN_NAME="kosync_bin"

cleanup() {
    echo "Cleaning up..."
    if [ -n "$PID" ]; then
        kill $PID 2>/dev/null || true
    fi
    rm -f "$DB" "$DB-shm" "$DB-wal" "$BIN_NAME"
}

trap cleanup EXIT

# Build server
echo "Building KOSYNC..."
go build -o $BIN_NAME ../cmd/kosync

# Create user via CLI
echo "Creating test user..."
CLI_OUTPUT=$(KOSYNC_DATABASE_PATH=$DB ./$BIN_NAME create-user "$USER" --password-stdin <<EOF
$PASS
EOF
)
if [[ $CLI_OUTPUT != *"User '$USER' created successfully."* ]]; then
    echo "CLI user creation FAILED: $CLI_OUTPUT"
    exit 1
fi

# Start server in background
echo "Starting KOSYNC server..."
export KOSYNC_PORT=$PORT
export KOSYNC_DATABASE_PATH=$DB
export KOSYNC_LOG_LEVEL=debug
export KOSYNC_DISABLE_REGISTRATION=true

./$BIN_NAME > server.log 2>&1 &
PID=$!
sleep 2

echo "Testing Auth..."
curl -s -X GET $URL/users/auth -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Accept: application/vnd.koreader.v1+json" > /dev/null

echo "Testing Auth Failure..."
curl -s -X GET $URL/users/auth -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:wrongpass" -H "Accept: application/vnd.koreader.v1+json" > /dev/null

echo "Testing 404..."
curl -s -X GET $URL/notfound -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Accept: application/vnd.koreader.v1+json" > /dev/null

echo "Testing Progress Update..."
curl -s -X PUT $URL/syncs/progress -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Content-Type: application/json" -H "Accept: application/vnd.koreader.v1+json" -d "{\"document\":\"$DOC\",\"percentage\":0.5,\"progress\":\"loc1\"}" > /dev/null

echo "Testing Progress Retrieval..."
RESP=$(curl -s -X GET $URL/syncs/progress/$DOC -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Accept: application/vnd.koreader.v1+json")

if [[ $RESP == *'"percentage":0.5'* ]]; then
    echo "Integration test PASSED"
    echo "--- Server Logs ---"
    cat server.log
else
    echo "Integration test FAILED: $RESP"
    echo "--- Server Logs ---"
    cat server.log
    exit 1
fi
