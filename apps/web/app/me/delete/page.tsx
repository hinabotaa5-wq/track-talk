"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";
import { unsubscribePushOnThisDevice } from "@/lib/sign-out";
import { clearHomeFeeds, clearSavedFeeds } from "@/lib/home-feed-cache";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

const confirmPhrase = "退会する";
export default function DeleteAccountPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const leaving = useRef(false);
  const [phrase, setPhrase] = useState("");
  const [error, setError] = useState("");
  const { pending: submitting, acquire, release } = useActionLock();
  const [authLeft, setAuthLeft] = useState(false);

  useEffect(() => {
    if (!loading && !session && !leaving.current) router.replace("/login");
  }, [loading, session, router]);

  async function leave() {
    leaving.current = true;
    // 退会で消えた投稿が、ホームの一覧に残らないようにする。
    clearHomeFeeds();
    clearSavedFeeds();
    await unsubscribePushOnThisDevice(null);
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

  const ready = phrase === confirmPhrase;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">退会</h1>
        <p className="text-sm text-slate-300">退会すると、次のデータが削除され、元に戻せません。</p>
      </div>
      <ul className="list-disc space-y-1 pl-5 text-sm text-slate-300">
        <li>あなたの投稿、コメント、レビュー、いいね、保存、通知</li>
        <li>感覚メモと競技会の記録、エグ練大会のエントリーと投票</li>
        <li>ケガのグループへの参加、回答、段階の記録と一言</li>
        <li>練習への参加と評価。あなたが作った練習の投稿も削除されます</li>
        <li>お問い合わせ（商品のリクエストを含みます）、コーチの応募と顔写真、あなたが送った通報</li>
        <li>ログインに使うアカウント</li>
      </ul>
      <p className="text-sm text-slate-300">
        他の人の投稿は残ります。あなたの投稿についたコメントも、投稿といっしょに削除されます。オンラインコーチングの提供中は、あなたの指導の契約とやりとりも削除されます。
      </p>
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
