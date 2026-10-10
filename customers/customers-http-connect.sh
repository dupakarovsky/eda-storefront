#!/usr/bin/env bash
# customers/http-connect.sh
set -euo pipefail

CUSTOMERS_URL="http://localhost:8084/customers.v1.CustomersService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

echo "== RegisterCustomer =="
REGISTER_RESP=$(curl -s "${HEADERS[@]}" -d '{"name":"  Verification Customer  ","smsNumber":"+5531999990000"}' "$CUSTOMERS_URL/RegisterCustomer")
echo "$REGISTER_RESP" | jq .
CUSTOMER_ID=$(echo "$REGISTER_RESP" | jq -r '.customer.id')
echo "$REGISTER_RESP" | jq -e '.customer.name == "Verification Customer" and .customer.enabled == true' >/dev/null && echo "trim + enabled-by-default OK" || echo "trim/enabled MISMATCH"

echo "== GetCustomer =="
curl -s "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/GetCustomer" | jq .

echo "== ChangeSmsNumber =="
curl -s "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\",\"smsNumber\":\"+5531888880000\"}" "$CUSTOMERS_URL/ChangeSmsNumber" | jq .

echo "== ChangeSmsNumber to the same number (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\",\"smsNumber\":\"+5531888880000\"}" "$CUSTOMERS_URL/ChangeSmsNumber"

echo "== ChangeSmsNumber blank (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\",\"smsNumber\":\"   \"}" "$CUSTOMERS_URL/ChangeSmsNumber"

echo "== AuthorizeCustomer (enabled, expect {} / HTTP 200) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/AuthorizeCustomer"

echo "== EnableCustomer while already enabled (expect FailedPrecondition / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/EnableCustomer"

echo "== DisableCustomer =="
curl -s "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/DisableCustomer" | jq .

echo "== DisableCustomer while already disabled (expect FailedPrecondition / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/DisableCustomer"

echo "== AuthorizeCustomer (disabled, expect PermissionDenied / HTTP 403) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/AuthorizeCustomer"

echo "== EnableCustomer =="
curl -s "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/EnableCustomer" | jq .

echo "== AuthorizeCustomer (re-enabled, expect HTTP 200) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"customerId\":\"$CUSTOMER_ID\"}" "$CUSTOMERS_URL/AuthorizeCustomer"

echo "== RegisterCustomer with blank name (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"name":"  ","smsNumber":"+5531999990000"}' "$CUSTOMERS_URL/RegisterCustomer"

echo "== RegisterCustomer with blank sms number (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"name":"Verification Customer","smsNumber":""}' "$CUSTOMERS_URL/RegisterCustomer"

echo "== GetCustomer against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"customerId":"does-not-exist"}' "$CUSTOMERS_URL/GetCustomer"

echo "== AuthorizeCustomer against unknown ID (expect NotFound / HTTP 404) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d '{"customerId":"does-not-exist"}' "$CUSTOMERS_URL/AuthorizeCustomer"
