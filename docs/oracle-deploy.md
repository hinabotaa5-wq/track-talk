# API を Oracle Cloud（Always Free）で動かす

いまの API の公開先は Render です。この手順は、あとから VM に移すときのためのものです。

Go API を Oracle の VM で動かし、画面は Vercel のままにする構成です。
VM では Docker 上で API と Caddy（HTTPS）が動きます。
無料枠の条件は変わるので、作成前に [Always Free の公式ページ](https://docs.oracle.com/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm) を確認してください。

```
ブラウザ ─ Vercel（画面）
   └────── https://api.<ドメイン> ─ Caddy ─ API（Docker）─ Supabase（DB・Auth）
```

## 1. VM を作る（手作業）

1. Oracle Cloud に登録する。**ホームリージョンは東京か大阪**（後から変更できない）
2. Compute で VM を作る。イメージは Ubuntu 24.04
   - `VM.Standard.A1.Flex`（Arm、2 OCPU / 12 GB まで無料）が取れればそちら。「out of host capacity」になるときは `VM.Standard.E2.1.Micro`（AMD、1 GB）で足りる
   - パブリック IP を付ける。SSH 公開鍵を登録する
3. VCN の Security List（または NSG）の Ingress に **TCP 80、TCP 443、UDP 443** を `0.0.0.0/0` で追加する
4. ドメインの A レコード（例 `api.example.com`）を VM のパブリック IP に向ける

## 2. サーバーを初期設定する

```bash
scp deploy/setup.sh ubuntu@<IP>:
ssh ubuntu@<IP> bash setup.sh
```

Docker、セキュリティ更新の自動適用（再起動は日本時間 04:00）、メモリが少ない VM 用のスワップ、`/opt/track-talk`、年2回の `importmajors` の cron が入ります。

続けて `.env` を置きます。

```bash
scp deploy/.env.example ubuntu@<IP>:/opt/track-talk/.env
ssh ubuntu@<IP> nano /opt/track-talk/.env
```

入れる値は `deploy/.env.example` です。`API_IMAGE` の `OWNER` は GitHub のユーザー名を小文字にし、`CORS_ORIGINS` は Vercel の本番 URL にします。

## 3. GitHub に登録する（手作業）

デプロイ専用の鍵を作り、公開鍵だけを VM の `~/.ssh/authorized_keys` に追加します。

```bash
ssh-keygen -t ed25519 -f deploy_key -N "" -C track-talk-deploy
ssh-keyscan -t ed25519 <IP>
```

VM へ自動で送る workflow を戻したときに、リポジトリの Settings → Secrets and variables → Actions へ登録します。

| Secret | 値 |
| --- | --- |
| `DEPLOY_HOST` | VM のパブリック IP |
| `DEPLOY_USER` | `ubuntu` |
| `DEPLOY_SSH_KEY` | `deploy_key`（秘密鍵）の中身 |
| `DEPLOY_KNOWN_HOSTS` | `ssh-keyscan` の出力 |

登録後は `deploy_key` をローカルから消してください。

## 4. 初回デプロイと切り替え

1. VM 上でイメージを取得して `docker compose up -d` する。Render を使っているあいだは、そのための GitHub Actions は置いていない
2. `https://api.<ドメイン>/health` が `{"status":"ok"}` を返すことを確認する
3. Vercel の `NEXT_PUBLIC_API_BASE_URL` を `https://api.<ドメイン>` にしてデプロイする
4. 画面からログインと投稿ができることを確認する

## 運用

```bash
ssh ubuntu@<IP>
cd /opt/track-talk
docker compose logs -f api         # ログ
docker compose restart api         # 再起動
docker compose ps                  # 状態
```

- **ロールバック**: `.env` の `API_IMAGE` をコミット SHA のタグ（`...-api:<sha>`）にして `docker compose up -d`
- **監視**: UptimeRobot などで `https://api.<ドメイン>/health` を5分間隔で見て、メールで通知する。何も入れないと、止まっても気づけない
- **回収対策**: Oracle は、7日間 CPU（上位5%）と通信がともに 20% 未満（Arm はメモリも）の VM を回収することがある。軽い API は AMD・Arm どちらでも該当しやすい。Oracle の案内では、アカウントを **Pay As You Go に切り替える**と回収されず、Always Free の範囲内なら課金されない。カードは登録済みなので、公開前に切り替えておくのを勧める。切り替え後も、Always Free の上限を超える構成（大きい Arm、追加ディスクなど）は作らないこと
- **主要大会の取り込み**: 手動なら `docker compose --profile jobs run --rm importmajors`

## 注意

- `DEPLOY_USER` は docker グループに入るため、実質 root 権限です。鍵の管理に気をつけてください
- レート制限は API のプロセス内で数えています。VM を複数台に増やすと台数ぶん緩くなります
