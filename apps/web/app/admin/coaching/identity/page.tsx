"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachingGate } from "@/components/coaching-gate";
import { useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, apiErrorText } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { CoachIdentity } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useState, type FormEvent } from "react";

// コーチの特商法の登録情報(本名・住所・電話番号)を、開示請求や紛争の対応のために見る。
// 見るたびに目的を記録する。画面に出したままにしないよう、再読み込みで消える。

type Viewed = {
  identity: CoachIdentity;
  required: boolean;
  access_logs: { id: string; purpose: string; created_at: string; admin_nickname: string | null }[];
};

function IdentityAdmin() {
  const { token } = useAuth();
  const [coachID, setCoachID] = useState("");
  const [purpose, setPurpose] = useState("");
  const [viewed, setViewed] = useState<Viewed | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const confirmDialog = useConfirm();
  const { pending, acquire, release } = useActionLock();
  const path = `/v1/admin/coaching/coaches/${encodeURIComponent(coachID.trim())}`;

  async function onView(e: FormEvent) {
    e.preventDefault();
    if (!acquire()) return;
    setError(""); setMessage(""); setViewed(null);
    try {
      setViewed(await apiFetch<Viewed>(`${path}/commerce-identity`, { method: "POST", token, body: JSON.stringify({ purpose }) }));
    } catch (err) {
      setError(apiErrorText(err, "読み込めませんでした"));
    } finally {
      release();
    }
  }

  async function forceRequired() {
    if (!(await confirmDialog({ title: "このコーチを表記要に戻しますか？", body: "事業としての実態が見えた場合に使います。戻したあと、コーチは自分で表記不要に申告し直せません。", confirmLabel: "表記要に戻す" }))) return;
    if (!acquire()) return;
    setError(""); setMessage("");
    try {
      await apiFetch(`${path}/commerce-required`, { method: "POST", token });
      setMessage("表記要に戻しました");
      setViewed((v) => (v ? { ...v, required: true } : v));
    } catch (err) {
      setError(apiErrorText(err, "変更できませんでした"));
    } finally {
      release();
    }
  }

  return <div className="mx-auto max-w-md space-y-5">
    <h1 className="text-2xl font-bold">コーチの特商法の登録情報</h1>
    <p className="text-sm text-slate-300">開示請求・紛争の対応のときだけ見てください。見た日時・目的・管理者が記録されます。</p>
    <form onSubmit={onView} className="space-y-2">
      <label className="block text-sm">コーチのID<Input value={coachID} onChange={(e) => setCoachID(e.target.value)} aria-label="コーチのID" /></label>
      <label className="block text-sm">見る目的<Input value={purpose} maxLength={200} onChange={(e) => setPurpose(e.target.value)} placeholder="例: 利用者からの開示請求(お問い合わせ 10/28)" aria-label="見る目的" /></label>
      <Button type="submit" disabled={pending || !coachID.trim() || !purpose.trim()}>記録して見る</Button>
    </form>
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    {message ? <p role="status" className="text-sm text-blue-200">{message}</p> : null}
    {viewed ? <section className="space-y-3 rounded-xl border border-white/20 p-3 text-sm">
      <dl className="space-y-1">
        <div><dt className="text-xs text-slate-300">氏名</dt><dd>{viewed.identity.legal_name}</dd></div>
        <div><dt className="text-xs text-slate-300">住所</dt><dd>{viewed.identity.address}</dd></div>
        <div><dt className="text-xs text-slate-300">電話番号</dt><dd>{viewed.identity.phone}</dd></div>
        <div><dt className="text-xs text-slate-300">表記</dt><dd>{viewed.required ? "表記する" : "表記しない(本人の申告)"}</dd></div>
      </dl>
      {viewed.required ? null : <Button type="button" disabled={pending} onClick={() => void forceRequired()}>表記要に戻す</Button>}
      <h2 className="font-semibold">閲覧の記録</h2>
      <ul className="space-y-1 text-xs text-slate-300">{viewed.access_logs.map((l) => <li key={l.id}>{formatDateTime(l.created_at)} · {l.admin_nickname ?? "管理者"} · {l.purpose}</li>)}</ul>
    </section> : null}
  </div>;
}

export default function Page() { return <CoachingGate><IdentityAdmin /></CoachingGate>; }
