#!/usr/bin/env bash
# contract-evolution/price-verification.sh

# What to expect
# AddProduct: the response now has "description": "a priced widget", which the old stores dropped. The catalogs show 10.00 | <seq> in both services, with the sequence number of the ProductAdded event.
# The two price events: each stream message ends with a data object holding both keys, like "data":{"delta":2.5,"price":12.5} and "data":{"delta":-1.25,"price":11.25}. Seeing price in the raw stream message is the main evidence that the publisher change is live.
# Catalogs after the changes: 11.25 in both, with a last_applied_seq higher than after AddProduct.
# The two rejections: each prints HTTP 400 with an invalid_argument body mentioning two decimal places. The catalogs afterwards must show exactly the same price and sequence as before, since a rejected call publishes nothing.
# The decrease to zero: the jq output of the API response probably has no price key at all, because proto3 JSON omits a zero-valued number (the same behavior the customers contract documents for false). That’s a client-side display quirk and not a bug. The stream message, though, must still contain "price":0, and both catalogs must show 0.00. This is the live version of the decrease to zero test.

set -euo pipefail

STORES_URL="http://localhost:8080/stores.v1.StoresService"
HEADERS=(-H "Content-Type: application/json" -H "Connect-Protocol-Version: 1")

set -a
source .postgres.env
set +a
export PGPASSWORD="$EDA_DEV_PASSWORD"

catalog_price() {
    psql -h 127.0.0.1 -p 5433 -U eda_dev -d "$1" -At -F' | ' \
        -c "SELECT price, last_applied_seq FROM catalog_products WHERE id = '$2'"
}

show_catalogs() {
    echo "baskets (price | last_applied_seq): $(catalog_price basketsdb "$1")"
    echo "depot   (price | last_applied_seq): $(catalog_price depotdb "$1")"
}

last_event() {
    docker run --rm --network eda-storefront_eda-net natsio/nats-box:latest \
        nats --server nats://nats:4222 stream get stores --last-for "$1" | grep -E "^Item:|aggregate_id"
}

echo "== [stores] CreateStore =="
STORE_ID=$(curl -s "${HEADERS[@]}" -d '{"name":"Price Verification Grocer","location":"1 Price Ave"}' "$STORES_URL/CreateStore" | jq -r '.store.id')
echo "store: $STORE_ID"

echo "== [stores] AddProduct at 10.00 (expect the description to come back) =="
ADD_RESP=$(curl -s "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Price Widget\",\"description\":\"a priced widget\",\"sku\":\"SKU-PRICE-1\",\"price\":10.00}" "$STORES_URL/AddProduct")
echo "$ADD_RESP" | jq .
PRODUCT_ID=$(echo "$ADD_RESP" | jq -r '.product.id')
sleep 2
show_catalogs "$PRODUCT_ID"

echo "== [stores] IncreaseProductPrice to 12.50, then DecreaseProductPrice to 11.25 =="
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"price\":12.50}" "$STORES_URL/IncreaseProductPrice" | jq -c .product.price
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"price\":11.25}" "$STORES_URL/DecreaseProductPrice" | jq -c .product.price
sleep 2

echo "== stream: last ProductPriceIncreased and ProductPriceDecreased (expect delta AND price) =="
last_event "stores.product.ProductPriceIncreased"
last_event "stores.product.ProductPriceDecreased"

echo "== catalogs (expect 11.25 in both, sequences higher than before) =="
show_catalogs "$PRODUCT_ID"

echo "== [stores] IncreaseProductPrice to 13.005 (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"price\":13.005}" "$STORES_URL/IncreaseProductPrice"

echo "== [stores] AddProduct at 3.999 (expect InvalidArgument / HTTP 400) =="
curl -s -o /dev/stderr -w "HTTP %{http_code}\n" "${HEADERS[@]}" -d "{\"storeId\":\"$STORE_ID\",\"name\":\"Bad Price\",\"description\":\"x\",\"sku\":\"SKU-PRICE-2\",\"price\":3.999}" "$STORES_URL/AddProduct"

sleep 1
echo "== catalogs after the rejected calls (expect identical to above: no event was published) =="
show_catalogs "$PRODUCT_ID"

echo "== [stores] DecreaseProductPrice to 0 (the price key must still be in the event) =="
curl -s "${HEADERS[@]}" -d "{\"productId\":\"$PRODUCT_ID\",\"price\":0}" "$STORES_URL/DecreaseProductPrice" | jq -c .product
sleep 2
last_event "stores.product.ProductPriceDecreased"
show_catalogs "$PRODUCT_ID"
