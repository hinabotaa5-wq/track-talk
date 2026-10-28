"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { CoachApplication } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// コーチだけをやめる（審査中なら応募を取り下げる）。アカウントは残る。
// アカウントごと消す退会は /me/delete で、こことは分けている。

const confirmPhrase = "コーチをやめる";

export default function CoachWithdrawPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [application, setApplication] = useState<CoachApplication | null | undefined>(undefined);
  const [phrase, setPhrase] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const { pending: submitting, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
      .then((d) => {
        if (!cancelled) setApplication(d.application);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (phrase !== confirmPhrase || !session || !acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me/withdraw", {
        method: "POST",
        token,
        body: JSON.stringify({ confirm: confirmPhrase }),
      });
      setApplication(d.application);
      setDone(true);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.confirm || err.message : "コーチをやめられませんでした");
    } finally {
      release();
    }
  }

  if (loading || !session || (application === undefined && !error)) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  if (done) {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">コーチをやめました</h1>
        <p className="text-sm text-slate-300">
          アカウントはそのまま使えます。もう一度コーチとして活動したくなったら、応募すると審査からやり直せます。
        </p>
        <Link href="/" className="block text-sm text-blue-300 underline">
          ホームへ
        </Link>
      </div>
    );
  }

  const active = application?.status === "approved" || application?.status === "pending";
  if (!active) {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">コーチをやめる</h1>
        {error ? <p className="text-sm text-red-400">{error}</p> : <p className="text-sm text-slate-300">いまはコーチとして活動していません。</p>}
        <Link href="/me" className="block text-sm text-blue-300 underline">
          マイページへ
        </Link>
      </div>
    );
  }

  const pendingOnly = application?.status === "pending";
  const ready = phrase === confirmPhrase;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">{pendingOnly ? "コーチの応募を取り下げる" : "コーチをやめる"}</h1>
        <p className="text-sm text-slate-300">
          コーチだけをやめます。アカウントは残り、投稿や記録もそのままで、一般の利用者として使い続けられます。
        </p>
      </div>
      <ul className="list-disc space-y-1 pl-5 text-sm text-slate-300">
        {pendingOnly ? (
          <li>審査中の応募を取り下げます</li>
        ) : (
          <>
            <li>コーチ一覧、プラン、空き状況から外れ、新しい依頼は届かなくなります</li>
            <li>まだ返事をしていない添削の打診は、ほかのコーチに回ります</li>
            <li>まだ送金していない報酬は、最低額に届かなくても翌月にまとめて送金します</li>
          </>
        )}
        <li>顔写真は削除します</li>
        <li>もう一度コーチになるには、応募して審査を受け直します</li>
      </ul>
      {pendingOnly ? null : (
        <p className="text-sm text-slate-300">
          進行中のパーソナル指導、添削中の依頼、対応中の追加相談があるあいだはやめられません。終わってから手続きしてください。
        </p>
      )}
      <form onSubmit={onSubmit} className="space-y-3">
        <label className="block space-y-1 text-sm">
          <span className="text-slate-300">確認のため「{confirmPhrase}」と入力</span>
          <Input value={phrase} onChange={(event) => setPhrase(event.target.value)} autoComplete="off" />
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" variant="danger" className="w-full" disabled={!ready || submitting}>
          {pendingOnly ? "応募を取り下げる" : "コーチをやめる"}
        </Button>
      </form>
      <p className="text-xs text-slate-400">
        アカウントごと削除したいときは、
        <Link href="/me/delete" className="text-blue-300 underline">
          退会
        </Link>
        の手続きをしてください。
      </p>
    </div>
  );
}
