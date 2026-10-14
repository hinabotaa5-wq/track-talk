"use client";

import { useAuth } from "@/components/auth-provider";
import { GoogleSignInButton } from "@/components/google-sign-in-button";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import type { Session } from "@supabase/supabase-js";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

const confirmPhrase = "退会する";
const recentLoginMs = 5 * 60 * 1000;

function signedInWithGoogle(session: Session) {
  return session.user.app_metadata?.provider === "google";
}

function recentLogin(accessToken: string | null) {
  if (!accessToken) return false;
  const part = accessToken.split(".")[1];
  if (!part) return false;
  try {
    const base = part.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base + "=".repeat((4 - (base.length % 4)) % 4);
    const json = JSON.parse(atob(padded)) as {
      auth_time?: number;
      amr?: { timestamp?: number }[];
    };
    let latest = typeof json.auth_time === "number" ? json.auth_time : 0;
    for (const item of json.amr ?? []) {
      if (typeof item.timestamp === "number" && item.timestamp > latest) latest = item.timestamp;
    }
    if (!latest) return false;
    return Date.now() - latest * 1000 <= recentLoginMs;
  } catch {
    return false;
  }
}

export default function DeleteAccountPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const leaving = useRef(false);
  const [phrase, setPhrase] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();
  const [authLeft, setAuthLeft] = useState(false);
  const google = session ? signedInWithGoogle(session) : false;
  const fresh = google && recentLogin(token);

  useEffect(() => {
    if (!loading && !session && !leaving.current) router.replace("/login");
  }, [loading, session, router]);

  async function leave() {
    leaving.current = true;
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.replace("/");
    router.refresh();
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (phrase !== confirmPhrase || !session || !acquire()) return;
    setError("");
    try {
      let accessToken = token;
      if (!google) {
        const email = session.user.email;
        const supabase = createClient();
        if (!email || !supabase) {
          setError("もう一度ログインできません。");
          release();
          return;
        }
        const { data, error: signInError } = await supabase.auth.signInWithPassword({ email, password });
        if (signInError || !data.session) {
          setError(signInError ? authErrorMessage(signInError) : "もう一度ログインできません。");
          release();
          return;
        }
        accessToken = data.session.access_token;
      } else if (!fresh) {
        setError("退会する前にもう一度ログインしてください");
        release();
        return;
      }
      const result = await apiFetch<{ auth_deleted: boolean }>("/v1/me", {
        method: "DELETE",
        token: accessToken,
        body: JSON.stringify({ confirm: confirmPhrase }),
      });
      if (!result.auth_deleted) {
        setAuthLeft(true);
        return;
      }
      await leave();
    } catch (err) {
      if (err instanceof ApiRequestError) {
        setError(err.fields?.confirm || err.message);
      } else {
        setError("退会に失敗しました");
      }
      release();
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  if (authLeft) {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">退会</h1>
        <p className="text-sm text-slate-300">
          投稿・コメントなどのデータは削除しました。ログイン用アカウントは削除できなかったため、同じメールアドレスで再びログインできることがあります。
        </p>
        <Button type="button" className="w-full" onClick={() => void leave()}>
          ログアウトしてホームへ
        </Button>
      </div>
    );
  }

  const ready = phrase === confirmPhrase && (google ? fresh : password.length > 0);

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">退会</h1>
        <p className="text-sm text-slate-300">退会すると、次のデータが削除され、元に戻せません。</p>
      </div>
      <ul className="list-disc space-y-1 pl-5 text-sm text-slate-300">
        <li>あなたの投稿、コメント、いいね、保存、通知</li>
        <li>練習への参加と評価。あなたが作った練習の投稿も削除されます</li>
        <li>お問い合わせと、あなたが送った通報</li>
        <li>ログインに使うアカウント</li>
      </ul>
      <p className="text-sm text-slate-300">
        他の人の投稿は残ります。あなたの投稿についたコメントも、投稿といっしょに削除されます。
      </p>
      <p className="text-sm text-slate-300">
        退会の直前に、もう一度ログインしてください。ログインから5分を過ぎた状態では退会できません。
      </p>
      {google && !fresh ? (
        <GoogleSignInButton nextPath="/me/delete" label="Googleで再ログイン" />
      ) : null}
      <form onSubmit={onSubmit} className="space-y-3">
        {google ? null : (
          <label className="block space-y-1 text-sm">
            <span className="text-slate-300">パスワード</span>
            <Input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
            />
          </label>
        )}
        <label className="block space-y-1 text-sm">
          <span className="text-slate-300">確認のため「退会する」と入力</span>
          <Input
            value={phrase}
            onChange={(event) => setPhrase(event.target.value)}
            autoComplete="off"
          />
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" variant="danger" className="w-full" disabled={!ready || submitting}>
          退会する
        </Button>
      </form>
    </div>
  );
}
