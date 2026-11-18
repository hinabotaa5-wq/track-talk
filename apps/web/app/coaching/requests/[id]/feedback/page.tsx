"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { PageLoading } from "@/components/page-loading";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { L } from "@/lib/i18n";
import type { ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";

// 返却後に任意で書ける匿名の感想（本体 #911）。コーチには7日後に本文だけが届く。
// 送らなくても他の操作（追加質問など）は何も止めない。
const feedbackMaxLength = 500;

function FeedbackForm({ request, token, onSubmitted }: { request: ReviewRequest; token: string | null; onSubmitted: () => void }) {
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const trimmed = body.trim();

  async function submit() {
    if (!trimmed || !acquire()) return;
    setError("");
    try {
      await apiFetch(`/v1/coaching/requests/${request.id}/feedback`, { method: "POST", token, body: JSON.stringify({ body: trimmed }) });
      onSubmitted();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }

  return (
    <section className="space-y-2">
      <h2 className="text-sm font-semibold">{L("この添削はどうでしたか（任意）")}</h2>
      <textarea
        aria-label={L("この添削はどうでしたか（任意）")}
        value={body}
        maxLength={feedbackMaxLength}
        rows={4}
        disabled={pending}
        onChange={(e) => setBody(e.target.value)}
        className="w-full rounded-xl border border-white/25 bg-white/10 p-3 text-sm"
      />
      <p className="text-xs text-slate-300">{L("感想は匿名で、7日後にコーチへ届きます。どの添削への感想かはコーチには分かりません。")}</p>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="button" disabled={pending || !trimmed} onClick={() => void submit()}>{L("感想を送る")}</Button>
    </section>
  );
}

export default function FeedbackPage() {
  const { session, token, loading } = useAuth();
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [eligible, setEligible] = useState<{ request: ReviewRequest; token: string } | null>(null);
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (loading) return;
    if (!session || !token || !id) {
      router.replace("/coaching");
      return;
    }
    let active = true;
    void apiFetch<ReviewRequest>(`/v1/coaching/requests/${id}`, { token }).then((request) => {
      if (!active) return;
      if (request.response && (request.status === "returned" || request.status === "clarifying") && !request.feedback_submitted) {
        setSubmitted(false);
        setEligible({ request, token });
      } else router.replace("/coaching");
    }).catch(() => {
      if (active) router.replace("/coaching");
    });
    return () => { active = false; };
  }, [loading, session, token, id, router]);

  if (loading || !session || !eligible || eligible.request.id !== id || eligible.token !== token) {
    return <PageLoading screen label={L("読み込み中...")} />;
  }
  return (
    <div className="mx-auto max-w-md space-y-5">
      {submitted ? <p className="text-sm text-slate-200">{L("感想を送りました")}</p> :
        <FeedbackForm request={eligible.request} token={token} onSubmitted={() => setSubmitted(true)} />}
    </div>
  );
}
