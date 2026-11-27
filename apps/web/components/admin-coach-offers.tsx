"use client";
import { L } from "@/lib/i18n";

import { Button } from "@/components/ui/button";
import { apiErrorText, apiFetch } from "@/lib/api";
import type { AdminCoachOfferApplication, CoachDemoVideo } from "@/lib/coach-offer";
import { formatDateTime } from "@/lib/format";
import { useCallback, useEffect, useState } from "react";

// コーチの添削項目・見本動画の申請の一覧と承認（本体 #1086）。価格申請の管理画面に同居させる。
// 申請はすべて承認する方針（却下はしない。承認は確認・記録の場）。承認の前に、追加項目のタイトルと見本動画を
// 変更前（いま有効な内容）→変更後で見比べられるようにする（補足 S3）。
const statusLabels: Record<AdminCoachOfferApplication["status"], string> = {
  get pending() { return L("承認待ち"); },
  get approved() { return L("承認済み・切り替え待ち"); },
  get applied() { return L("有効"); },
};

function Videos({ videos }: { videos: readonly CoachDemoVideo[] }) {
  if (videos.length === 0) return <p className="text-xs text-slate-300">{L("見本動画なし")}</p>;
  return (
    <div className="flex flex-wrap gap-2">
      {videos.map((v, i) => (
        <video key={`${i}-${v.url}`} src={v.url} controls playsInline preload="metadata" aria-label={L("見本動画 {n}", { n: i + 1 })} className="w-32 max-h-[160px] rounded-lg bg-black" />
      ))}
    </div>
  );
}

function Items({ items }: { items: readonly string[] }) {
  if (items.length === 0) return <p className="text-xs text-slate-300">{L("追加項目なし")}</p>;
  return (
    <ol className="list-decimal space-y-0.5 pl-5 break-words">
      {items.map((item) => <li key={item}>{item}</li>)}
    </ol>
  );
}

export function AdminCoachOffers({ token }: { token: string | null | undefined }) {
  const [rows, setRows] = useState<AdminCoachOfferApplication[] | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  const fetchRows = useCallback(
    () => apiFetch<{ applications: AdminCoachOfferApplication[] }>("/v1/admin/coaching/coach-offers", { token }).then((d) => d.applications),
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
      await apiFetch(`/v1/admin/coaching/coach-offers/${id}/approve`, { method: "POST", token });
      setRows(await fetchRows());
    } catch (e) {
      setError(apiErrorText(e, L("承認できませんでした")));
    } finally {
      setPending(false);
    }
  }

  return (
    <section aria-label={L("添削項目・見本動画の申請")} className="space-y-3">
      <h2 className="text-xl font-bold">{L("添削項目・見本動画の申請")}</h2>
      <p className="text-sm">{L("申請はすべて承認します。承認の前に、追加項目と見本動画の変更前・変更後を確かめてください。承認しても、承認の時点でそのコーチが対応中だった依頼がすべて終わるまでは、いまの内容のままです。")}</p>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {rows?.length === 0 ? <p className="text-sm text-slate-200">{L("申請はありません")}</p> : null}
      <ul className="space-y-3">
        {rows?.map((r) => (
          <li key={r.id} className="space-y-2 rounded-xl border border-white/20 p-3 text-sm" data-testid={`offer-application-${r.id}`}>
            <p className="font-semibold">{r.coach_name || L("名前なし")}</p>
            <p>{statusLabels[r.status]} · {L("申請 {v0}", { v0: formatDateTime(r.updated_at) })}</p>
            {r.status !== "applied" ? (
              <div className="space-y-1" data-testid="offer-before">
                <p className="text-xs font-semibold text-slate-300">{L("変更前（いま有効な内容）")}</p>
                <Items items={r.current_items} />
                <Videos videos={r.current_demo_videos} />
              </div>
            ) : null}
            <div className="space-y-1" data-testid="offer-after">
              <p className="text-xs font-semibold text-slate-300">{r.status === "applied" ? L("切り替えた内容") : L("変更後（申請の内容）")}</p>
              <Items items={r.items} />
              <Videos videos={r.demo_videos} />
            </div>
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
    </section>
  );
}
