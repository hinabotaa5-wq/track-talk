"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiErrorText, apiFetch } from "@/lib/api";
import { coachPricePlanLabels, type AdminCoachPriceApplication } from "@/lib/coach-price";
import { formatDateTime, formatYen } from "@/lib/format";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

// コーチの価格申請の一覧と承認（本体 #1052）。申請はすべて承認する方針（却下はしない。承認は確認・記録の場）。
// 承認しても即時には切り替わらず、承認の時点でそのコーチが対応中だった依頼がすべて終わってから有効になる。
const statusLabels: Record<AdminCoachPriceApplication["status"], string> = {
  get pending() { return L("承認待ち"); },
  get approved() { return L("承認済み・切り替え待ち"); },
  get applied() { return L("有効"); },
};

const sourceLabels: Record<AdminCoachPriceApplication["source"], string> = {
  get price() { return L("コーチ画面から"); },
  get coach_apply() { return L("応募フォームの希望価格"); },
};

export default function CoachPricesAdmin() {
  const { token } = useAuth();
  const [rows, setRows] = useState<AdminCoachPriceApplication[] | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  const fetchRows = useCallback(
    () => apiFetch<{ applications: AdminCoachPriceApplication[] }>("/v1/admin/coaching/coach-prices", { token }).then((d) => d.applications),
    [token],
  );

  useEffect(() => {
    if (!token) return;
    let live = true;
    fetchRows()
      .then((list) => { if (live) setRows(list); })
      .catch((e) => { if (live) setError(apiErrorText(e, L("読み込めませんでした"))); });
    return () => { live = false; };
  }, [token, fetchRows]);

  async function approve(id: string) {
    if (pending) return;
    setPending(true);
    setError("");
    try {
      await apiFetch(`/v1/admin/coaching/coach-prices/${id}/approve`, { method: "POST", token });
      setRows(await fetchRows());
    } catch (e) {
      setError(apiErrorText(e, L("承認できませんでした")));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="mx-auto max-w-lg space-y-4">
      <h1 className="text-2xl font-bold">{L("コーチの価格申請")}</h1>
      <p className="text-sm">{L("申請はすべて承認します。承認しても、承認の時点でそのコーチが対応中だった依頼がすべて終わるまでは、いまの価格のままです。")}</p>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {rows?.length === 0 ? <p className="text-sm text-slate-200">{L("申請はありません")}</p> : null}
      <ul className="space-y-3">
        {rows?.map((r) => (
          <li key={r.id} className="space-y-1 rounded-xl border border-white/20 p-3 text-sm">
            <p className="font-semibold">
              {r.coach_name || L("名前なし")} · {coachPricePlanLabels[r.plan]} · {formatYen(r.price_yen)}
            </p>
            <p>
              {statusLabels[r.status]} · {sourceLabels[r.source]} · {L("申請 {v0}", { v0: formatDateTime(r.updated_at) })}
            </p>
            <p className="text-xs text-slate-300">
              {L("いまの価格: {v0}", { v0: r.current_price_yen != null ? formatYen(r.current_price_yen) : L("未設定") })}
            </p>
            {r.approved_at ? (
              <p className="text-xs text-slate-300">
                {L("承認 {v0}（{v1}）", { v0: formatDateTime(r.approved_at), v1: r.approved_by_nickname ?? L("不明") })}
                {r.applied_at ? ` · ${L("切り替え {v0}", { v0: formatDateTime(r.applied_at) })}` : ""}
              </p>
            ) : null}
            {r.status === "pending" ? (
              <Button type="button" disabled={pending} onClick={() => void approve(r.id)}>{L("承認する")}</Button>
            ) : null}
          </li>
        ))}
      </ul>
      <Link href="/admin" className="text-blue-300 underline">{L("管理画面へ")}</Link>
    </div>
  );
}
