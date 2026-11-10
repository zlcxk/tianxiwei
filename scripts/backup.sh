#!/usr/bin/env bash
# 备份 PostgreSQL 数据库到 backups/ 目录（自定义格式，适合 pg_restore）。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

POSTGRES_USER="${POSTGRES_USER:-xiwei}"
POSTGRES_DB="${POSTGRES_DB:-xiwei}"

mkdir -p backups
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="backups/xiwei-${STAMP}.dump"
TMP="${OUT}.partial"

cleanup() {
  rm -f "$TMP"
}
trap cleanup EXIT

echo "正在备份 ${POSTGRES_DB} ..."
docker compose exec -T postgres pg_dump \
  -U "$POSTGRES_USER" \
  -d "$POSTGRES_DB" \
  --format=custom \
  --no-owner \
  --no-privileges > "$TMP"

mv "$TMP" "$OUT"
echo "备份完成：$OUT"
echo "大小：$(wc -c < "$OUT") 字节"
echo "恢复命令：scripts/restore.sh $OUT"
