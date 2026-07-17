#!/usr/bin/env sh
# Builds the throwaway repo the demo tapes record against, so every recording
# starts from identical state: two worktrees (shop on main, shop-payments on a
# feature branch) and two environments (dev — the default — and staging), both
# worktrees on dev and in sync.
#
# Requires `envkeep` on PATH (record.sh puts ./bin first). Safe to re-run.
set -e

ROOT=/tmp/envkeep-demo
rm -rf "$ROOT"
mkdir -p "$ROOT"
cd "$ROOT"

git init -q shop
cd shop
git config user.email demo@envkeep.dev
git config user.name demo
git commit -q --allow-empty -m "init"
git worktree add -q ../shop-payments -b payments

cat > .env <<'EOF'
# shop — local settings
PORT=3000
DATABASE_URL=postgres://localhost:5432/shop_dev
REDIS_URL=redis://localhost:6379/0
STRIPE_KEY=sk_test_demo_1234
EOF
envkeep use -c dev >/dev/null

cat > .env <<'EOF'
# shop — local settings
PORT=3000
DATABASE_URL=postgres://db.staging.internal:5432/shop
REDIS_URL=redis://cache.staging.internal:6379/0
STRIPE_KEY=sk_test_demo_9999
EOF
envkeep use -c staging >/dev/null

# Back to dev; the staging snapshot stays in its vault.
envkeep use dev >/dev/null

# The payments worktree starts on dev too, .env materialized from the vault.
cd ../shop-payments
envkeep use dev >/dev/null

echo "demo repo ready at $ROOT (shop + shop-payments, envs: dev*, staging)"
