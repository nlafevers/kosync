#!/bin/bash
# Integration test for KOSYNC
set -e

# Config
PORT=8081
URL="http://localhost:$PORT"
DB="kosync.db"
USER="testuser"
PASS="testpass"
DOC="doc123"

# Start server in background
go build -o kosync ./cmd/kosync
./kosync &
PID=$!
trap "kill $PID; rm -f $DB" EXIT
sleep 2

echo "Testing Registration..."
curl -s -X POST $URL/users/create -H "Content-Type: application/json" -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" > /dev/null

echo "Testing Auth..."
curl -s -X GET $URL/users/auth -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Accept: application/vnd.koreader.v1+json" > /dev/null

echo "Testing Progress Update..."
curl -s -X PUT $URL/syncs/progress -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Content-Type: application/json" -H "Accept: application/vnd.koreader.v1+json" -d "{\"document\":\"$DOC\",\"percentage\":0.5,\"progress\":\"loc1\"}" > /dev/null

echo "Testing Progress Retrieval..."
RESP=$(curl -s -X GET $URL/syncs/progress/$DOC -H "X-AUTH-USER:$USER" -H "X-AUTH-KEY:$PASS" -H "Accept: application/vnd.koreader.v1+json")

if [[ $RESP == *'"percentage":0.5'* ]]; then
    echo "Integration test PASSED"
else
    echo "Integration test FAILED: $RESP"
    exit 1
fi
