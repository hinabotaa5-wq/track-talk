"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

const confirmPhrase = "退会する";
export default function DeleteAccountPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const leaving = useRef(false);
  const [phrase, setPhrase] = useState("");
  const [keepInjury, setKeepInjury] = useState(false);
  const [error, setError] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();
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
    if (phrase !== confirmPhrase || !session || !acquire()) return;
    setError("");
    try {
      const result = await apiFetch<{ auth_deleted: boolean }>("/v1/me", {
        method: "DELETE",
        token,
        body: JSON.stringify({ confirm: confirmPhrase, keep_injury_anonymized: keepInjury }),
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

  const ready = phrase === confirmPhrase;

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
      <label className="flex items-start gap-2 rounded-xl border border-white/20 glass bg-white/8 p-3 text-sm text-slate-200">
        <input
          type="checkbox"
          className="mt-1"
          checked={keepInjury}
          onChange={(event) => setKeepInjury(event.target.checked)}
        />
        <span className="space-y-1">
          <span className="block font-semibold text-slate-100">ケガの記録を、匿名にして残す(任意)</span>
          <span className="block text-slate-300">
            ケガ人の溜まり場での、部位・段階の移り変わり・答えた状況・投稿の文章を、あなたと結びつかない形にして残し、統計や研究に使います。メールアドレスやリンクなどは取り除きますが、文章の中の名前や地名は取り除けないことがあります。残した記録は、退会後に探し出して削除できません。チェックしなければ、何も残りません。
          </span>
        </span>
      </label>
      <form onSubmit={onSubmit} className="space-y-3">
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
