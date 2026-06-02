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
TMP_BODY=$(mktemp)

# Assertion helpers
assert_status() {
    local expected="$1"
    local actual="$2"
    local label="$3"
    if [ "$actual" = "$expected" ]; then
        echo "PASS [$label]: HTTP $actual"
    else
        echo "FAIL [$label]: expected HTTP $expected, got HTTP $actual"
        echo "--- Server Logs ---"
        cat server.log
        exit 1
    fi
}

assert_content_type() {
    local actual="$1"
    local label="$2"
    if [[ "$actual" == *"application/vnd.koreader.v1+json"* ]]; then
        echo "PASS [$label content-type]: $actual"
    else
        echo "FAIL [$label content-type]: expected application/vnd.koreader.v1+json, got '$actual'"
        echo "--- Server Logs ---"
        cat server.log
        exit 1
    fi
}

# curl_check runs a curl request, storing body in TMP_BODY.
# Sets globals: RESP_CODE and RESP_CTYPE.
curl_check() {
    local out
    out=$(curl -s -o "$TMP_BODY" -w "%{http_code}|%{content_type}" "$@")
    RESP_CODE="${out%%|*}"
    RESP_CTYPE="${out#*|}"
}

cleanup() {
    echo "Cleaning up..."
    if [ -n "$PID" ]; then
        kill $PID 2>/dev/null || true
    fi
    rm -f "$DB" "$DB-shm" "$DB-wal" "$BIN_NAME" "$TMP_BODY" server.log
}

trap cleanup EXIT

# Build server
echo "Building KOSYNC..."
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOCACHE=/tmp/kosync-gocache go build -o $BIN_NAME "$SCRIPT_DIR/../cmd/kosync"

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
echo "PASS [CLI create-user]"

# Start server in background
echo "Starting KOSYNC server..."
export KOSYNC_PORT=$PORT
export KOSYNC_DATABASE_PATH=$DB
export KOSYNC_LOG_LEVEL=debug
export KOSYNC_DISABLE_REGISTRATION=true
export KOSYNC_RATE_LIMIT_ENABLED=false

./$BIN_NAME > server.log 2>&1 &
PID=$!
sleep 2

# Auth success: valid credentials → 200 with KOReader content type
echo "Testing Auth Success..."
curl_check -X GET "$URL/users/auth" \
    -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" \
    -H "Accept: application/vnd.koreader.v1+json"
assert_status 200 "$RESP_CODE" "GET /users/auth valid"
assert_content_type "$RESP_CTYPE" "GET /users/auth valid"

# Auth failure: wrong key → 401
echo "Testing Auth Failure (wrong key)..."
curl_check -X GET "$URL/users/auth" \
    -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:wrongpass" \
    -H "Accept: application/vnd.koreader.v1+json"
assert_status 401 "$RESP_CODE" "GET /users/auth wrong key"

# Auth failure: missing auth headers → 401
echo "Testing Auth Failure (missing headers)..."
curl_check -X GET "$URL/users/auth" \
    -H "Accept: application/vnd.koreader.v1+json"
assert_status 401 "$RESP_CODE" "GET /users/auth no headers"

# Progress update (PUT) → 200 with KOReader content type
echo "Testing Progress Update..."
curl_check -X PUT "$URL/syncs/progress" \
    -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" \
    -H "Content-Type: application/json" \
    -H "Accept: application/vnd.koreader.v1+json" \
    -d "{\"document\":\"$DOC\",\"percentage\":0.5,\"progress\":\"loc1\"}"
assert_status 200 "$RESP_CODE" "PUT /syncs/progress"
assert_content_type "$RESP_CTYPE" "PUT /syncs/progress"

# Progress retrieval (GET) → 200 with KOReader content type
echo "Testing Progress Retrieval..."
curl_check -X GET "$URL/syncs/progress/$DOC" \
    -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" \
    -H "Accept: application/vnd.koreader.v1+json"
assert_status 200 "$RESP_CODE" "GET /syncs/progress/$DOC"
assert_content_type "$RESP_CTYPE" "GET /syncs/progress/$DOC"
if [[ $(cat "$TMP_BODY") != *'"percentage":0.5'* ]]; then
    echo "FAIL [progress body]: expected '\"percentage\":0.5' in response"
    echo "--- Body ---"
    cat "$TMP_BODY"
    echo "--- Server Logs ---"
    cat server.log
    exit 1
fi
echo "PASS [progress body]: found percentage 0.5"

# Progress not found (GET unknown doc with valid auth) → 404
echo "Testing Progress Not Found..."
curl_check -X GET "$URL/syncs/progress/unknowndoc" \
    -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" \
    -H "Accept: application/vnd.koreader.v1+json"
assert_status 404 "$RESP_CODE" "GET /syncs/progress/unknowndoc"

echo ""
echo "Integration test PASSED"
echo "--- Server Logs ---"
cat server.log
