"use client";
import { L } from "@/lib/i18n";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime, formatYen, todayInTokyo } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useState } from "react";

// 継続プランのコーチ不在日の合意延長（本体 #898）。
// コーチが事前に申告した不在期間について、選手（未成年は保護者も）とコーチの全員が明示的に同意したときだけ、
// 契約の終了日を不在の日数分だけ後ろ倒しにする。追加料金はなく、価格と内容は変わらない。
// 契約チャットが見られなくなった後も、この表示から合意の内容を確認できる。

export type Absence = {
  id: string;
  status: "declared" | "agreed" | "withdrawn" | "declined" | "lapsed";
  absent_from: string;
  return_on: string;
  absent_days: number;
  reason: string;
  minor: boolean;
  price_yen: number;
  original_ends_at: string;
  proposed_ends_at: string;
  refund_estimate_yen: number;
  player_consented_at: string | null;
  guardian_consented_at: string | null;
  coach_consented_at: string | null;
  agreed_at: string | null;
  agreed_original_ends_at: string | null;
  agreed_new_ends_at: string | null;
};

type AbsenceList = { absences: Absence[]; eligible: boolean; max_total_days: number };

// "2026-11-20" を日本の日付の表示にする。
export function absenceDay(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  if (!y || !m || !d) return day;
  return L("{y}年{m}月{d}日", { y, m, d });
}

// 契約の終了日（終了の時刻の1秒前の日）の表示。
export function absenceEndDay(iso: string): string {
  return absenceDay(todayInTokyo(new Date(new Date(iso).getTime() - 1000)));
}

const refundTiming = L("終了の手続きをした時点で、お支払いに使ったカードへ返金します。明細に反映されるまでの日数はカード会社によって異なります。");

function Offer({ a, role, busy, onAct }: { a: Absence; role: "requester" | "coach" | "guardian"; busy: boolean; onAct: (who: "consent" | "coach-consent", agree: boolean) => void }) {
  const mine =
    role === "coach" ? a.coach_consented_at : role === "guardian" ? a.guardian_consented_at : a.player_consented_at;
  return (
    <div className="space-y-3 rounded-xl border border-amber-400/40 bg-amber-500/10 p-3 text-sm text-slate-100">
      <p className="font-bold text-white">{L("コーチの不在日について、ご確認ください")}</p>
      <p>
        {L("{from}から{to}まで（復帰は{ret}・{days}日間）、コーチが対応できません。理由: {reason}", {
          from: absenceDay(a.absent_from),
          to: absenceDay(new Date(new Date(`${a.return_on}T00:00:00+09:00`).getTime() - 86400000).toISOString().slice(0, 10)),
          ret: absenceDay(a.return_on),
          days: a.absent_days,
          reason: a.reason,
        })}
      </p>
      <div className="grid gap-2">
        <div className="space-y-1 rounded-lg border border-white/20 bg-white/10 p-2">
          <p className="font-semibold text-white">{L("案1: 契約を延長する")}</p>
          <p>
            {L("終了日が{from}から{to}に変わります。追加料金はなく、価格（{price}）と内容は変わりません。", {
              from: absenceEndDay(a.original_ends_at),
              to: absenceEndDay(a.proposed_ends_at),
              price: formatYen(a.price_yen),
            })}
          </p>
        </div>
        <div className="space-y-1 rounded-lg border border-white/20 bg-white/10 p-2">
          <p className="font-semibold text-white">{L("案2: 延長せずに終了する")}</p>
          <p>
            {L("今日終了した場合の返金の概算は{yen}です（未経過分の日割り・全額返金、解約料なし）。", { yen: formatYen(a.refund_estimate_yen) })}
            {refundTiming}
          </p>
        </div>
      </div>
      <ul className="list-disc space-y-1 pl-5 text-xs text-slate-200">
        <li>{L("延長した後も、いつでも契約を終了できます。")}</li>
        <li>{L("延長は原則1回・合計7日までです（運営の方針）。再度の不在や、復帰が遅れた場合は、あらためて申告と合意が必要で、元の合意では延びません。")}</li>
        <li>{L("返事をしない・見ただけでは、延長されません。選手とコーチの双方の同意があったときだけ延長します。")}</li>
        {a.minor ? <li>{L("18歳未満の契約では、保護者の同意も必要です。")}</li> : null}
      </ul>
      <p className="text-xs text-slate-300">
        {L("同意の状況: 選手 {p}／コーチ {c}{g}", {
          p: a.player_consented_at ? L("同意済み") : L("未同意"),
          c: a.coach_consented_at ? L("同意済み") : L("未同意"),
          g: a.minor ? L("／保護者 {v0}", { v0: a.guardian_consented_at ? L("同意済み") : L("未同意") }) : "",
        })}
      </p>
      {mine ? (
        <p className="text-xs font-semibold text-emerald-300">{L("あなたは同意済みです。")}</p>
      ) : (
        <div className="flex flex-wrap gap-2">
          <Button type="button" disabled={busy} onClick={() => onAct(role === "coach" ? "coach-consent" : "consent", true)}>
            {role === "coach" ? L("変更後の期間に同意する") : L("延長する")}
          </Button>
          <button
            type="button"
            disabled={busy}
            onClick={() => onAct(role === "coach" ? "coach-consent" : "consent", false)}
            className="min-h-11 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 disabled:opacity-50"
          >
            {role === "coach" ? L("申し出を取り下げる") : L("延長しない（元の終了日のまま）")}
          </button>
        </div>
      )}
    </div>
  );
}

function Declare({ onDone, base, token }: { onDone: () => void; base: string; token: string | null }) {
  const { pending, acquire, release } = useActionLock();
  const [open, setOpen] = useState(false);
  const [from, setFrom] = useState("");
  const [ret, setRet] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  async function submit() {
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(`${base}/absences`, { method: "POST", token, body: JSON.stringify({ absent_from: from, return_on: ret, reason }) });
      setOpen(false);
      setFrom("");
      setRet("");
      setReason("");
      onDone();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }
  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)} className="min-h-11 w-full rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100">
        {L("対応できない日を申告する")}
      </button>
    );
  }
  return (
    <div className="space-y-2 rounded-xl border border-white/20 bg-white/10 p-3 text-sm text-slate-100">
      <p className="font-bold text-white">{L("対応できない日の申告")}</p>
      <p className="text-xs text-slate-300">
        {L("判明した時点で事前に申告してください。復帰予定日は不在日に含めません。延長は選手とコーチの同意があったときだけで、原則1回・合計7日までです。病名や診断などの詳細は書かないでください。")}
      </p>
      <label className="block space-y-1">
        <span className="text-xs font-semibold">{L("不在の開始日")}</span>
        <Input type="date" value={from} min={todayInTokyo()} onChange={(e) => setFrom(e.target.value)} />
      </label>
      <label className="block space-y-1">
        <span className="text-xs font-semibold">{L("復帰予定日")}</span>
        <Input type="date" value={ret} min={from || todayInTokyo()} onChange={(e) => setRet(e.target.value)} />
      </label>
      <label className="block space-y-1">
        <span className="text-xs font-semibold">{L("理由の概要（体調不良・私的事情・災害など）")}</span>
        <Input value={reason} maxLength={200} onChange={(e) => setReason(e.target.value)} />
      </label>
      {error ? <p className="text-xs text-red-400">{error}</p> : null}
      <div className="flex gap-2">
        <Button type="button" disabled={pending || !from || !ret || !reason.trim()} onClick={() => void submit()}>
          {L("申告して延長を申し出る")}
        </Button>
        <button type="button" onClick={() => setOpen(false)} className="min-h-11 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100">
          {L("やめる")}
        </button>
      </div>
    </div>
  );
}

// 契約画面（選手・コーチ）と保護者の画面に出す。履歴も申告もなければ（コーチの申告ボタン以外は）何も出さない。
export function CoachingAbsence({ contract, token, guardianToken }: { contract: CoachingContract; token: string | null; guardianToken?: string }) {
  const base = guardianToken ? `/v1/guardian/${encodeURIComponent(guardianToken)}/contracts/${contract.id}` : `/v1/coaching/contracts/${contract.id}`;
  const role: "requester" | "coach" | "guardian" = guardianToken ? "guardian" : contract.role === "coach" ? "coach" : "requester";
  const [data, setData] = useState<AbsenceList | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  const load = useCallback(() => {
    apiFetch<AbsenceList>(`${base}/absences`, guardianToken ? undefined : { token })
      .then(setData)
      .catch(() => setData(null));
  }, [base, token, guardianToken]);
  useEffect(() => {
    load();
  }, [load]);

  async function act(id: string, who: "consent" | "coach-consent", agree: boolean) {
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(`${base}/absences/${id}/${who}`, { method: "POST", token: guardianToken ? undefined : token, body: JSON.stringify({ agree }) });
      load();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }

  if (!data) return null;
  const open = data.absences.find((a) => a.status === "declared");
  const agreed = data.absences.find((a) => a.status === "agreed");
  const canDeclare = role === "coach" && data.eligible && !open && contract.status === "active";
  if (!open && !agreed && !canDeclare) return null;
  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4" aria-label={L("コーチ不在日の延長")}>
      <h2 className="font-semibold">{L("コーチ不在日の延長")}</h2>
      {agreed ? (
        <div className="space-y-1 text-sm text-slate-100">
          <p className="font-semibold text-emerald-300">{L("不在日の延長に合意しています")}</p>
          <p>
            {L("不在{days}日間（{from}〜{to}）。契約の終了日は{old}から{next}に変わりました。追加料金はなく、価格と内容は変わりません。", {
              days: agreed.absent_days,
              from: absenceDay(agreed.absent_from),
              to: absenceDay(new Date(new Date(`${agreed.return_on}T00:00:00+09:00`).getTime() - 86400000).toISOString().slice(0, 10)),
              old: absenceEndDay(agreed.agreed_original_ends_at ?? agreed.original_ends_at),
              next: absenceEndDay(agreed.agreed_new_ends_at ?? agreed.proposed_ends_at),
            })}
          </p>
          <p className="text-xs text-slate-300">
            {L("合意の日時: {v0}", { v0: agreed.agreed_at ? formatDateTime(agreed.agreed_at) : "" })}
            {" · "}
            {L("返金の計算では、不在日は経過日数に数えず、日割りの分母（期間の日数）は当初のままです。")}
          </p>
        </div>
      ) : null}
      {open ? <Offer a={open} role={role} busy={pending} onAct={(who, agree) => void act(open.id, who, agree)} /> : null}
      {canDeclare ? <Declare base={base} token={token} onDone={load} /> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}
