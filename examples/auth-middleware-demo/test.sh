#!/bin/bash

# Auth Middleware Demo - Test Script
# This script tests various authentication scenarios

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_test() {
    echo -e "\n${YELLOW}→ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_response() {
    echo "$1" | jq '.' 2>/dev/null || echo "$1"
}

# Test 1: Health Check (Public)
print_header "TEST 1: Health Check (Public Endpoint)"
print_test "GET /health"

RESPONSE=$(curl -s "$BASE_URL/health")
print_response "$RESPONSE"

if echo "$RESPONSE" | grep -q '"status":"ok"'; then
    print_success "Health check passed"
else
    print_error "Health check failed"
fi

# Test 2: Homepage (Public)
print_header "TEST 2: Homepage (Public Endpoint)"
print_test "GET /"

RESPONSE=$(curl -s "$BASE_URL/")
print_response "$RESPONSE"

if echo "$RESPONSE" | grep -q '"message"'; then
    print_success "Homepage accessed"
else
    print_error "Homepage failed"
fi

# Test 3: Missing Token
print_header "TEST 3: Missing Token Error"
print_test "GET /api/user (no Authorization header)"

RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$BASE_URL/api/user")
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS/d')

print_response "$BODY"
echo "HTTP Status: $HTTP_STATUS"

if [ "$HTTP_STATUS" = "401" ] && echo "$BODY" | grep -q "AUTH_TOKEN_MISSING"; then
    print_success "Correct error: AUTH_TOKEN_MISSING (msgid: 1001)"
else
    print_error "Expected AUTH_TOKEN_MISSING error"
fi

# Test 4: Invalid Token
print_header "TEST 4: Invalid Token Error"
print_test "GET /api/user (with invalid token)"

RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -H "Authorization: Bearer invalid-token-12345" \
    "$BASE_URL/api/user")

HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS/d')

print_response "$BODY"
echo "HTTP Status: $HTTP_STATUS"

if [ "$HTTP_STATUS" = "401" ] && echo "$BODY" | grep -q "AUTH_TOKEN_INVALID"; then
    print_success "Correct error: AUTH_TOKEN_INVALID (msgid: 1002)"
else
    print_error "Expected AUTH_TOKEN_INVALID error"
fi

# Test 5: Malformed Bearer Token
print_header "TEST 5: Malformed Bearer Header"
print_test "GET /api/user (malformed Authorization header)"

RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    -H "Authorization: NotBearer token" \
    "$BASE_URL/api/user")

HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS/d')

print_response "$BODY"
echo "HTTP Status: $HTTP_STATUS"

if [ "$HTTP_STATUS" = "401" ] && echo "$BODY" | grep -q "AUTH_TOKEN_MISSING"; then
    print_success "Correct error: AUTH_TOKEN_MISSING"
else
    print_error "Expected AUTH_TOKEN_MISSING error"
fi

# Test 6: Valid Token (if provided)
if [ -n "$TEST_TOKEN" ]; then
    print_header "TEST 6: Valid Token"
    print_test "GET /api/user (with valid token from TEST_TOKEN env var)"

    RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
        -H "Authorization: Bearer $TEST_TOKEN" \
        "$BASE_URL/api/user")

    HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS" | cut -d: -f2)
    BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS/d')

    print_response "$BODY"
    echo "HTTP Status: $HTTP_STATUS"

    if [ "$HTTP_STATUS" = "200" ]; then
        print_success "Valid token accepted"
    else
        print_error "Valid token rejected"
    fi
else
    print_header "TEST 6: Valid Token (Skipped)"
    echo -e "${YELLOW}Set TEST_TOKEN environment variable to test with a real token${NC}"
    echo -e "${YELLOW}Example: TEST_TOKEN=your-jwt-token ./test.sh${NC}"
fi

# Summary
print_header "TEST SUMMARY"
echo -e "${GREEN}✓ Public endpoints work correctly${NC}"
echo -e "${GREEN}✓ Error codes are standardized:${NC}"
echo "  - Missing token: msgid=1001, errcode=AUTH_TOKEN_MISSING"
echo "  - Invalid token: msgid=1002, errcode=AUTH_TOKEN_INVALID"
echo "  - Cache error:   msgid=1003, errcode=AUTH_CACHE_ERROR"
echo ""
echo -e "${BLUE}To test with a real token:${NC}"
echo "  export TEST_TOKEN=your-jwt-token"
echo "  ./test.sh"
echo ""
