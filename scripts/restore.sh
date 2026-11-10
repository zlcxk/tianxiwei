#!/usr/bin/env bash
# 从 backups/ 中的自定义格式备份恢复数据库。
# 用法：scripts/restore.sh backups/xiwei-YYYYmmdd-HHMMSS.dump
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FILE="${1:-}"
if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
  echo "用法：scripts/restore.sh <备份文件.dump>" >&2
  exit 1
fi

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

POSTGRES_USER="${POSTGRES_USER:-xiwei}"
POSTGRES_DB="${POSTGRES_DB:-xiwei}"

echo "即将使用 $FILE"
echo "覆盖数据库 ${POSTGRES_DB} 的全部现有数据，且不可撤销。"
printf "输入 yes 继续："
read -r answer
if [ "$answer" != "yes" ]; then
  echo "已取消。"
  exit 1
fi

echo "停止 API 与 Worker，避免恢复期间写入 ..."
docker compose stop api worker

restore_services() {
  echo "重新启动 API 与 Worker ..."
  docker compose start api worker
}
trap restore_services EXIT

echo "正在恢复 ..."
docker compose exec -T postgres pg_restore \
  -U "$POSTGRES_USER" \
  -d "$POSTGRES_DB" \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges < "$FILE"

echo "恢复完成。"
