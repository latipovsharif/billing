#!/bin/sh
# Provision the billing trial plan with a price in EVERY currency that
# cloudmarket-server may send. That set is the server's available_currencies
# catalog (migrations_authorization/..._available_currencies.up.sql) — the same
# list RegisterShop validates a shop's currency_code against before calling
# billing. A missing (currency, month) price makes subscription create return
# 400 "no price for <cur>/month".
#
# Idempotent: backed by POST /v1/admin/plans (upsert), safe to re-run.
#
# Usage:
#   BILLING_API_KEY=... PLAN_CODE=trial ./scripts/provision-trial-plan.sh
#
# PLAN_CODE MUST equal cloudmarket-server's BILLING_TRIAL_PLAN env.
set -eu

: "${BILLING_BASE_URL:=http://localhost:4000}"
: "${BILLING_API_KEY:?set BILLING_API_KEY}"
: "${PLAN_CODE:?set PLAN_CODE (must equal cloudmarket-server BILLING_TRIAL_PLAN)}"
: "${PLAN_NAME:=Trial}"
: "${TRIAL_DAYS:=14}"
: "${INTERVAL:=month}" # cloudmarket-server StartTrial always sends "month"
# Minor units (e.g. tiyin). 0 = free trial. The amount is stored on the
# subscription and used for the post-trial renewal invoice, so set the real
# per-currency price before trials convert (re-run this with real AMOUNT_<CUR>).
: "${AMOUNT:=0}"

# available_currencies catalog. Keep in sync with the server migration.
CURRENCIES="UZS USD EUR RUB KZT KGS TJS TMT GBP TRY CNY AED"

prices=""
for c in $CURRENCIES; do
	# allow per-currency override via env AMOUNT_UZS, AMOUNT_KZT, ...
	eval "amt=\${AMOUNT_$c:-$AMOUNT}"
	prices="${prices:+$prices,}{\"currency\":\"$c\",\"interval\":\"$INTERVAL\",\"amount\":$amt}"
done

body="{\"code\":\"$PLAN_CODE\",\"name\":\"$PLAN_NAME\",\"trial_days\":$TRIAL_DAYS,\"prices\":[$prices]}"

echo "Provisioning plan '$PLAN_CODE' ($INTERVAL) for: $CURRENCIES"
curl -fsS -X POST \
	-H "Authorization: Bearer $BILLING_API_KEY" \
	-H "Content-Type: application/json" \
	-d "$body" \
	"$BILLING_BASE_URL/v1/admin/plans"
echo
echo "Done."
