#!/usr/bin/env bash
# Auth ユーザー(auth.users / auth.identities)だけをデータとして移すリハーサル。
# 旧 DB は読むだけ。新 DB の auth テーブルが空でなければ止まる。
# テーブルの定義は Supabase が新プロジェクトに作っているので、データだけを流す。
#   OLD_DATABASE_URL=... NEW_DATABASE_URL=... bash deploy/db-migration/rehearse-auth.sh
# 次は移さない: sessions・refresh_tokens・flow_state・one_time_tokens(使い捨て。新しい JWT 鍵では無効になる)、
# schema_migrations(Supabase が管理する)。
# 多要素認証(mfa_factors)を使っているときは TABLES に auth.mfa_factors を足す。
# dump にはパスワードのハッシュとメールアドレスが入る。DUMP は終わったら消すこと。
set -euo pipefail

: "${OLD_DATABASE_URL:?旧 DB の接続文字列を指定してください}"
: "${NEW_DATABASE_URL:?新 DB の接続文字列を指定してください}"
TABLES=${TABLES:-"auth.users auth.identities"}
DUMP=${DUMP:-$(mktemp -d)/tracktalk-auth.dump}

if [ "$OLD_DATABASE_URL" = "$NEW_DATABASE_URL" ]; then
  echo "旧と新が同じ DB です" >&2; exit 1
fi

count() { psql "$1" -Atc "select count(*) from $2"; }

for t in $TABLES; do
  n=$(count "$NEW_DATABASE_URL" "$t")
  if [ "$n" != "0" ]; then
    echo "新 DB の $t にすでに $n 行あります。空のプロジェクトを指定してください" >&2; exit 1
  fi
done

table_args=()
for t in $TABLES; do table_args+=(--table="$t"); done

echo "== dump (data only: $TABLES)"
pg_dump "$OLD_DATABASE_URL" --data-only --no-owner --no-privileges "${table_args[@]}" -Fc -f "$DUMP"
chmod 600 "$DUMP"
echo "== restore"
pg_restore --data-only --no-owner --no-privileges --exit-on-error -d "$NEW_DATABASE_URL" "$DUMP"

echo "== 行数の比較"
status=0
for t in $TABLES; do
  o=$(count "$OLD_DATABASE_URL" "$t"); n=$(count "$NEW_DATABASE_URL" "$t")
  if [ "$o" = "$n" ]; then echo "$t: 一致 ($n)"; else echo "$t: 旧 $o / 新 $n" >&2; status=1; fi
done
echo "dump: $DUMP （確認が済んだら rm で消すこと）"
exit $status
