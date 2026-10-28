# アカウントの守り(二段階認証・自動操作対策・変更通知メール)

Stripe のセキュリティ申告で挙げた対策の実装と、本番で有効にする順番。どれもコードは入っているが、設定を入れるまで本番では動かない。

## 1. 変更通知メール(Supabase の設定だけ)

Supabase の Authentication → Emails → Security notifications で、次を有効にして本文を貼る(件名は h1 と同じ)。

| 通知 | テンプレート |
| --- | --- |
| Password changed | `apps/web/emails/password-changed.html` |
| Email address changed | `apps/web/emails/email-address-changed.html` |
| MFA factor enrolled | `apps/web/emails/mfa-factor-enrolled.html` |
| MFA factor unenrolled | `apps/web/emails/mfa-factor-unenrolled.html` |

パスワード変更の通知は、2026-10-26 に本番で有効化済み(Codex が設定)。

## 2. 二段階認証(認証アプリ)

2026-10-28 に公開(マイページの入口を戻した)。パスキーは Supabase で正式に出てから足す。

- 利用者は マイページ → 二段階認証(`/me/security`)で設定・解除する。設定した人は、ログインのたびに6桁のコードを求められる(パスワードでも Google でも)。
- API も、設定済みの人(Supabase の `auth.mfa_factors` に `verified` がある人)がコードを入れる前のログイン(JWT の `aal` が `aal1`)で呼んだら、ログインが要る操作を 403 `mfa_required` で断る。ログインなしで読めるものは、ログインしていない人として返す。画面で止めるだけだと、パスワードだけ知った人がトークンで直に API を呼べてしまうため(`apps/api/internal/httpx/mfa_enforce.go`)。
  - 設定済みかどうかは1人1分だけ覚えて、毎回は DB に聞かない。
  - `auth.mfa_factors` が無い・読めない DB(手元やテストの素の Postgres)では調べない。本番の API がそうなっていたら、ログに `mfa enforcement disabled` が出る(正しく動いていれば最初の利用で `mfa enforcement enabled`)。
- `STAFF_MFA_REQUIRED=1` の API は、管理者と承認済みコーチの操作を、二段階認証を済ませたログイン(`aal2`)でしか受け付けない(403 `mfa_required`)。画面は `/v1/me` の `mfa_required` を見て、管理・コーチの画面の前に設定・確認を求める。

管理者・コーチに必須にする順番:

1. Supabase の Authentication → Multi-Factor で TOTP が有効なことを確かめる(既定で有効)。
2. Web を出し直し、管理者・コーチ全員が `/me/security` で設定する。
3. API の `STAFF_MFA_REQUIRED` を `1` にする。

## 3. 自動操作対策(Cloudflare Turnstile)

登録・ログイン・パスワード再設定・メールアドレス/パスワードの変更で、Turnstile の確認を出し、トークンを Supabase に渡す。

2026-10-27 に本番で有効化済み(ウィジェットのホスト名は `track-talk.netlify.app`、公開用の鍵は `netlify.toml`、秘密の鍵は Supabase だけ)。独自ドメインに移るときは、先に Cloudflare のウィジェットにそのホスト名を足す。

本番で有効にする順番(逆にすると、全員がログインできなくなる):

1. Cloudflare で Turnstile のウィジェットを作り、ホスト名に本番のドメインを入れる。
2. Web のビルド環境に `NEXT_PUBLIC_TURNSTILE_SITE_KEY` を入れて出し直す。確認が出ることを確かめる。
3. Supabase の Authentication → Attack Protection で CAPTCHA protection を有効にし、Turnstile とシークレットキーを入れる。
