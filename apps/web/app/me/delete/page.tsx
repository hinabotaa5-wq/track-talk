"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { createClient } from "@/lib/supabase";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

const confirmPhrase = "退会する";

export default function DeleteAccountPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const leaving = useRef(false);
  const [phrase, setPhrase] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [authLeft, setAuthLeft] = useState(false);

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
    if (phrase !== confirmPhrase || submitting) return;
    setError("");
    setSubmitting(true);
    try {
      const result = await apiFetch<{ auth_deleted: boolean }>("/v1/me", {
        method: "DELETE",
        token,
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
      setSubmitting(false);
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-zinc-600">読み込み中...</p>;
  }

  if (authLeft) {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">退会</h1>
        <p className="text-sm text-zinc-700">
          投稿・コメントなどのデータは削除しました。ログイン用アカウントは削除できなかったため、同じメールアドレスで再びログインできることがあります。
        </p>
        <Button type="button" className="w-full" onClick={() => void leave()}>
          ログアウトしてホームへ
        </Button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">退会</h1>
        <p className="text-sm text-zinc-700">退会すると、次のデータが削除され、元に戻せません。</p>
      </div>
      <ul className="list-disc space-y-1 pl-5 text-sm text-zinc-700">
        <li>あなたの投稿、コメント、いいね、保存、通知</li>
        <li>練習への参加と評価。あなたが作った練習の投稿も削除されます</li>
        <li>お問い合わせと、あなたが送った通報</li>
        <li>ログインに使うアカウント</li>
      </ul>
      <p className="text-sm text-zinc-700">
        他の人の投稿は残ります。あなたの投稿についたコメントも、投稿といっしょに削除されます。
      </p>
      <form onSubmit={onSubmit} className="space-y-3">
        <label className="block space-y-1 text-sm">
          <span className="text-zinc-700">確認のため「退会する」と入力</span>
          <Input
            value={phrase}
            onChange={(event) => setPhrase(event.target.value)}
            autoComplete="off"
          />
        </label>
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        <Button
          type="submit"
          variant="danger"
          className="w-full"
          disabled={phrase !== confirmPhrase || submitting}
        >
          退会する
        </Button>
      </form>
    </div>
  );
}
