"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachingGate } from "@/components/coaching-gate";
import { IdentitySession, identityFetch } from "@/components/identity-session";
import { useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { CoachIdentity } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useState, type FormEvent } from "react";

type Viewed = {
  identity: CoachIdentity;
  access_logs: { id: number; viewer: string; purpose: string; created_at: string }[];
};

// 身元情報を表示したら、本体の API に対象コーチの識別子と表示時刻だけを渡し、運営者へ閲覧通知のメールを送る（#431）。
// 身元情報の値は渡さない。通知は検知のための付随処理なので、失敗しても表示は止めない（API 側で失敗を記録する）。
function notifyIdentityView(coachID: string, viewedAt: string, mainToken: string | null): Promise<void> {
  return apiFetch<void>(`/v1/admin/coaching/coaches/${encodeURIComponent(coachID)}/identity-view-notice`, {
    method: "POST", token: mainToken, body: JSON.stringify({ viewed_at: viewedAt }),
  }).catch(() => undefined);
}

function IdentityAdmin({ token }: { token: string }) {
  const { token: mainToken } = useAuth();
  const [coachID, setCoachID] = useState("");
  const [purpose, setPurpose] = useState("");
  const [viewed, setViewed] = useState<Viewed | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const confirmDialog = useConfirm();
  const { pending, acquire, release } = useActionLock();
  const path = `/v1/admin/coaches/${encodeURIComponent(coachID.trim())}`;

  async function onView(e: FormEvent) {
    e.preventDefault();
    if (!acquire()) return;
    setViewed(null);
    setMessage("");
    setError("");
    try {
      const result = await identityFetch<Viewed>(`${path}/identity`, token, "POST", { purpose });
      setViewed(result);
      void notifyIdentityView(coachID.trim(), result.access_logs[0]?.created_at ?? new Date().toISOString(), mainToken);
    } catch (err) {
      setError(err instanceof Error ? err.message : "読み込めませんでした");
    } finally {
      release();
    }
  }

  async function changeDisclosure(force: boolean) {
    const title = force ? "このコーチを表記要に戻しますか？" : "このコーチの表記の公開を取り消しますか？";
    if (!(await confirmDialog({ title, body: "このコーチのプランを再公開するには、本人による公開設定が必要です。", confirmLabel: "変更する" })) || !acquire()) return;
    setMessage("");
    setError("");
    try {
      await identityFetch(`${path}/${force ? "require-disclosure" : "disclosures"}`, token, force ? "POST" : "DELETE");
      setMessage(force ? "表記要に戻しました" : "公開を取り消しました");
    } catch (err) {
      setError(err instanceof Error ? err.message : "変更できませんでした");
    } finally {
      release();
    }
  }

  return <div className="space-y-4">
    <p className="text-sm text-slate-300">開示請求・紛争の対応のときだけ見てください。見た日時・目的・閲覧者が記録されます。</p>
    <form method="post" onSubmit={onView} className="space-y-2">
      <label className="block text-sm">コーチのID<Input value={coachID} disabled={pending} onChange={(e) => {
        setCoachID(e.target.value); setViewed(null); setMessage("");
      }} required /></label>
      <label className="block text-sm">見る目的<Input value={purpose} onChange={(e) => setPurpose(e.target.value)} maxLength={200} required /></label>
      <Button type="submit" disabled={pending || !coachID.trim() || !purpose.trim()}>記録して見る</Button>
    </form>
    {coachID.trim() && <div className="flex flex-wrap gap-2">
      <Button type="button" variant="secondary" disabled={pending} onClick={() => void changeDisclosure(true)}>表記要に戻す</Button>
      <Button type="button" variant="secondary" disabled={pending} onClick={() => void changeDisclosure(false)}>公開を取り消す</Button>
    </div>}
    {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
    {message && <p role="status" className="text-sm text-blue-300">{message}</p>}
    {viewed && <section className="space-y-3 rounded-xl border border-white/20 p-3 text-sm">
      <dl className="space-y-1">
        <div><dt className="text-xs text-slate-300">氏名</dt><dd>{viewed.identity.legal_name}</dd></div>
        <div><dt className="text-xs text-slate-300">住所</dt><dd>{viewed.identity.address}</dd></div>
        <div><dt className="text-xs text-slate-300">電話番号</dt><dd>{viewed.identity.phone || "未登録"}</dd></div>
      </dl>
      <h2 className="font-semibold">閲覧の記録</h2>
      <ul className="space-y-1 text-xs text-slate-300">{viewed.access_logs.map((log) => (
        <li key={log.id}>{formatDateTime(log.created_at)} · {log.viewer} · {log.purpose}</li>
      ))}</ul>
    </section>}
  </div>;
}

export default function Page() {
  return <CoachingGate><main className="mx-auto max-w-md space-y-5">
    <h1 className="text-xl font-bold">コーチの特商法の登録情報</h1>
    <IdentitySession>{(token) => <IdentityAdmin token={token} />}</IdentitySession>
  </main></CoachingGate>;
}
