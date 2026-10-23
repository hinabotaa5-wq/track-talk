"use client";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, apiErrorText } from "@/lib/api";
import { consultationStatus, type Consultation } from "@/lib/coaching-consultations";
import { formatYen } from "@/lib/format";
import Link from "next/link";
import { useEffect, useState } from "react";
export default function ConsultationsAdmin() {
  const { token } = useAuth();
  const [rows, setRows] = useState<(Consultation & { requester_nickname: string | null; coach_nickname: string | null })[]>([]);
  const [error, setError] = useState(""); const [pending, setPending] = useState(false);
  useEffect(() => { if (!token) return; let live = true; apiFetch<{ consultations: typeof rows }>("/v1/admin/coaching/consultations", { token }).then((d) => { if (live) setRows(d.consultations); }).catch((e) => { if (live) setError(apiErrorText(e, "読み込めませんでした")); }); return () => { live = false; }; }, [token]);
  async function retry(id: string) { if (pending) return; setPending(true); try { await apiFetch(`/v1/admin/coaching/consultations/${id}/refund`, { method: "POST", token }); setError("返金の再試行を受け付けました。定期処理で照合します。"); } catch (e) { setError(apiErrorText(e, "再試行できませんでした")); } finally { setPending(false); } }
  return <div className="mx-auto max-w-lg space-y-4"><h1 className="text-2xl font-bold">追加相談・返金管理</h1><p className="text-sm">会話の内容は通報一覧からのみ確認できます。</p>{rows.map((r) => <article key={r.id} className="space-y-2 rounded-xl border border-white/20 p-3"><p className="text-sm">{r.requester_nickname ?? "退会済み"} → {r.coach_nickname ?? "退会済み"}</p><p>{consultationStatus[r.status]} · {formatYen(r.price_yen)} · {r.payment_state}</p><p className="break-all text-xs text-slate-300">{r.id}</p>{r.payment_state === "refund_pending" ? <Button disabled={pending} onClick={() => void retry(r.id)}>返金を照合・再試行</Button> : null}</article>)}{error ? <p role="status">{error}</p> : null}<Link href="/admin" className="text-blue-300 underline">管理画面へ</Link></div>;
}
