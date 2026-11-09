"use client";

import { FieldBlock } from "@/components/consult-fields";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import type { SupportConsultSettings } from "@/lib/support-consult";
import type { CoachApplication, CoachEvent } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useEffect, useState } from "react";

// 大会までプランの「相談の受付設定」。固定期間のプランを出品する代わりに、受付中かどうか、担当できる種目、
// 相談で聞きたいこと・サポートできる内容、価格の目安を設定する。期間・内容・価格は、相談のあとの提案で決める。
// 受け入れ状況は、対応可能カレンダーで閉じた期間だけで決まる（人数の上限は置かない）。
export function CoachConsultSettings({ coach, token }: { coach: CoachApplication; token: string | null }) {
  const [s, setS] = useState<SupportConsultSettings | null>(null);
  const [approved, setApproved] = useState(true);
  const [min, setMin] = useState("");
  const [max, setMax] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const canServe = (coach.service_types ?? []).includes("competition_support");

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ settings: SupportConsultSettings; disclosure_approved?: boolean }>("/v1/coach/me/support-consult", { token })
      .then((d) => {
        if (cancelled) return;
        setS(d.settings);
        setMin(d.settings.price_min_yen?.toString() ?? "");
        setMax(d.settings.price_max_yen?.toString() ?? "");
        setApproved(d.disclosure_approved !== false);
      })
      .catch(() => { if (!cancelled) setError("相談の受付設定を読み込めませんでした"); });
    return () => { cancelled = true; };
  }, [token]);

  async function save() {
    if (!s || !acquire()) return;
    setError("");
    setFields({});
    setSaved(false);
    const num = (v: string) => (v.trim() === "" ? null : Number(v.replace(/[,，円\s]/g, "")));
    try {
      const d = await apiFetch<{ settings: SupportConsultSettings }>("/v1/coach/me/support-consult", {
        method: "PUT", token, body: JSON.stringify({ ...s, price_min_yen: num(min), price_max_yen: num(max) }),
      });
      setS(d.settings);
      setSaved(true);
    } catch (e) {
      if (e instanceof ApiRequestError && e.fields) setFields(e.fields);
      else setError(e instanceof ApiRequestError ? e.message : "保存できませんでした");
    } finally {
      release();
    }
  }

  return (
    <section aria-label="大会までプランの相談の受付設定" className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <div className="space-y-1">
        <h3 className="font-bold">大会までプラン: 相談の受付設定</h3>
        <p className="text-xs text-slate-300">大会までプランは、固定の期間・価格では出品しません。選手が無料で相談してから、あなたが期間（開始日〜目標大会の日）・内容・価格を提案して始まります。運営手数料は20%です。</p>
      </div>
      {!canServe ? <p className="text-xs text-amber-300">先に、上の「担当するサービス」で大会までプランを登録してください。</p> : null}
      {s ? (
        <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); void save(); }}>
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={s.accepting} disabled={!canServe} onChange={(e) => setS({ ...s, accepting: e.target.checked })} />
            <span className="font-semibold">相談を受け付ける</span>
          </label>
          {fields.accepting ? <p className="text-sm text-red-400">{fields.accepting}</p> : null}
          <FieldBlock label="相談を受ける種目" error={fields.events}>
            <div className="flex flex-wrap gap-2">
              {(coach.events ?? []).map((ev: CoachEvent) => {
                const on = s.events.includes(ev);
                return (
                  <button key={ev} type="button" aria-pressed={on}
                    className={`rounded-full border px-3 py-1.5 text-sm ${on ? "border-blue-400 bg-blue-500/30 text-pure-white" : "border-white/25 bg-white/10 text-slate-200"}`}
                    onClick={() => setS({ ...s, events: on ? s.events.filter((x) => x !== ev) : [...s.events, ev] })}>{coachEventLabels[ev]}</button>
                );
              })}
            </div>
          </FieldBlock>
          <FieldBlock label="相談で聞きたいこと・サポートできる内容（1000文字まで）" error={fields.description}>
            <Textarea aria-label="相談で聞きたいこと・サポートできる内容" rows={4} maxLength={1000} value={s.description} onChange={(e) => setS({ ...s, description: e.target.value })} />
          </FieldBlock>
          <FieldBlock label="価格の目安（任意・円）。選手は相談の前に見られます。実際の価格は提案で決めます" error={fields.price_min_yen}>
            <div className="flex items-center gap-2">
              <Input aria-label="価格の目安の下限" inputMode="numeric" value={min} onChange={(e) => setMin(e.target.value)} />
              <span>〜</span>
              <Input aria-label="価格の目安の上限" inputMode="numeric" value={max} onChange={(e) => setMax(e.target.value)} />
            </div>
          </FieldBlock>
          <p className="text-xs text-slate-300">新しい相談を受けられない期間は、<Link href="/coach/availability" className="underline">カレンダー</Link>で「不可」にしてください。選手側には「受付停止中」と表示され、期間が終わると自動で「受付中」に戻ります。</p>
          {s.accepting && !approved ? <p className="text-xs text-amber-300">特商法の表記が承認されるまで、選手側には公開されません。<Link href="/coach/commerce" className="underline">特商法・身元</Link>で、このサービスの表記を承認してください。</p> : null}
          {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
          {saved ? <p role="status" className="text-sm text-emerald-300">保存しました</p> : null}
          <Button type="submit" disabled={pending}>受付設定を保存</Button>
        </form>
      ) : error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      <Link href="/coach/support-consults" className="block text-blue-300 underline">届いた相談を見る</Link>
    </section>
  );
}
