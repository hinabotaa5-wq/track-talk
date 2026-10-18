#!/usr/bin/env bash
# DB 移行のリハーサル: 旧 DB を dump して新 DB に restore し、行数とマイグレーション版を突き合わせる。
# 旧 DB は読むだけで、書き込むのは新 DB だけ。新 DB には必ずリハーサル用（空）を指定すること。
#   OLD_DATABASE_URL=... NEW_DATABASE_URL=... bash deploy/db-migration/rehearse.sh
# SCHEMAS に auth を足すと Auth ユーザーも移す（例: SCHEMAS="public auth"）。
set -euo pipefail

: "${OLD_DATABASE_URL:?旧 DB の接続文字列を指定してください}"
: "${NEW_DATABASE_URL:?新 DB の接続文字列を指定してください}"
SCHEMAS=${SCHEMAS:-public}
DUMP=${DUMP:-$(mktemp -d)/tracktalk.dump}

if [ "$OLD_DATABASE_URL" = "$NEW_DATABASE_URL" ]; then
  echo "旧と新が同じ DB です" >&2; exit 1
fi
newtables=$(psql "$NEW_DATABASE_URL" -Atc "select count(*) from information_schema.tables where table_schema = 'public'")
if [ "$newtables" != "0" ]; then
  echo "新 DB の public にすでにテーブルがあります（$newtables 個）。空のリハーサル用 DB を指定してください" >&2; exit 1
fi

schema_args=()
for s in $SCHEMAS; do schema_args+=(--schema="$s"); done

echo "== dump ($SCHEMAS)"
time pg_dump "$OLD_DATABASE_URL" --no-owner --no-privileges "${schema_args[@]}" -Fc -f "$DUMP"
echo "== restore"
# Supabase には public・auth が最初からあるので、スキーマ自体の作成とコメントは流さない
LIST=$(dirname "$DUMP")/restore.list
pg_restore -l "$DUMP" | grep -vE '^[0-9]+; [0-9]+ [0-9]+ (SCHEMA|COMMENT) - (public|auth)( |$)' > "$LIST"
time pg_restore --no-owner --no-privileges --exit-on-error -L "$LIST" -d "$NEW_DATABASE_URL" "$DUMP"

# 行数は pg_class の推定ではなく count(*) で数える
counts() {
  psql "$1" -Atc "select format('select %L, count(*) from %I.%I;', table_schema || '.' || table_name, table_schema, table_name)
    from information_schema.tables where table_type = 'BASE TABLE' and table_schema = any (string_to_array('${SCHEMAS// /,}', ',')) order by 1" \
    | psql "$1" -At -F ' ' | sort
}
echo "== 行数の比較"
old_counts=$(counts "$OLD_DATABASE_URL")
[ -n "$old_counts" ] || { echo "旧 DB の行数が取れませんでした" >&2; exit 1; }
diff <(echo "$old_counts") <(counts "$NEW_DATABASE_URL") && echo "行数: 一致 ($(echo "$old_counts" | wc -l | tr -d ' ') テーブル)"

echo "== マイグレーション版の比較"
v() { psql "$1" -Atc "select version, dirty from public.schema_migrations"; }
diff <(v "$OLD_DATABASE_URL") <(v "$NEW_DATABASE_URL") && echo "版: 一致 ($(v "$NEW_DATABASE_URL"))"
echo "dump: $DUMP"
