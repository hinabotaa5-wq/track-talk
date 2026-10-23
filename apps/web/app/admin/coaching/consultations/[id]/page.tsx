"use client";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, apiErrorText } from "@/lib/api";
import type { Consultation } from "@/lib/coaching-consultations";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
function ConsultationReport() {
  const { token } = useAuth(); const { id } = useParams<{ id: string }>(); const report = useSearchParams().get("report");
  const [c, setC] = useState<Consultation | null>(null); const [error, setError] = useState(""); const [pending, setPending] = useState(false);
  const path = `/v1/admin/coaching/consultations/${id}`; const query = `?report=${encodeURIComponent(report ?? "")}`;
  useEffect(() => { if (!token) return; let live = true; apiFetch<{ consultation: Consultation }>(path + query, { token }).then((d) => { if (live) setC(d.consultation); }).catch((e) => { if (live) setError(apiErrorText(e, "確認できませんでした")); }); return () => { live = false; }; }, [path, query, token]);
  async function hide() { if (pending) return; setPending(true); try { await apiFetch(`${path}/hide${query}`, { method: "POST", token }); setC((old) => old ? { ...old, hidden: true } : old); } catch (e) { setError(apiErrorText(e, "非表示にできませんでした")); } finally { setPending(false); } }
  return <div className="mx-auto max-w-lg space-y-4"><h1 className="text-2xl font-bold">通報された追加相談</h1><p className="text-xs text-slate-300">この画面を開いた記録を保存しています。</p>{c ? <><p className="whitespace-pre-wrap">質問：{c.question}</p>{c.video_url ? <video src={c.video_url} controls className="w-full" /> : null}<p className="whitespace-pre-wrap">回答：{c.answer}</p>{c.answer_video_url ? <video src={c.answer_video_url} controls className="w-full" /> : null}<p className="whitespace-pre-wrap">確認：{c.clarify_body}</p><p className="whitespace-pre-wrap">補足：{c.clarification}</p><Button disabled={pending || c.hidden} onClick={() => void hide()}>{c.hidden ? "非表示済み" : "非表示にして進行中の相談を停止する"}</Button></> : null}{error ? <p role="alert">{error}</p> : null}<Link href="/admin" className="text-blue-300 underline">管理画面へ</Link></div>;
}
export default function ConsultationReportPage() { return <Suspense fallback={<p>読み込み中…</p>}><ConsultationReport /></Suspense>; }
