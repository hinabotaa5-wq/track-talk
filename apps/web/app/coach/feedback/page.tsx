"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { apiFetch } from "@/lib/api";
import type { CoachFeedback } from "@/lib/types";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 選手からの匿名の感想（本体 #911）。送信から7日たったものの本文だけを並べる。
// どの添削・どの選手への感想かが分からないよう、日付・選手名・依頼へのリンクは出さない。
export default function CoachFeedbackPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [rows, setRows] = useState<CoachFeedback[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/coach/feedback")}`);
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ feedbacks: CoachFeedback[] }>("/v1/coach/feedback", { token })
      .then((d) => { if (!cancelled) setRows(d.feedbacks); })
      .catch(() => { if (!cancelled) setError(L("感想を読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);

  if (loading || !session || (rows === null && !error)) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <CoachSubpageShell title={L("選手からの感想")}>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {rows?.length === 0 ? <p className="text-sm text-slate-200">{L("まだ感想はありません。")}</p> : null}
      <ul className="space-y-2">
        {rows?.map((f) => (
          <li key={f.id} className="whitespace-pre-wrap break-words rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
            {f.body}
          </li>
        ))}
      </ul>
    </CoachSubpageShell>
  );
}
