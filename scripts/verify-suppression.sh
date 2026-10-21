#!/bin/sh
set -eu

base_url="${BASE_URL:-http://localhost:8080}"

curl --fail-with-body -sS -X POST "$base_url/events" \
  -H 'Content-Type: application/json' \
  -d '{"id":"subscriber-17-optout","kind":"subscriber_updated","phone":"+14155550117","opted_out":true}'
printf '\n'
curl --fail-with-body -sS -X POST "$base_url/events" \
  -H 'Content-Type: application/json' \
  -d '{"id":"asset-900-ready","kind":"asset_ready","phone":"+14155550117"}'
printf '\n'
