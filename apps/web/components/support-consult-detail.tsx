"use client";
import { L } from "@/lib/i18n";

import { SupportTermsNotice } from "@/components/support-terms-notice";
import { ContactNotice, useSolicitationGuard } from "@/components/contact-notice";
import { ContractConsentChecks, noConsents, type ContractConsents } from "@/components/coaching-contracts";
import { FieldBlock, selectClass } from "@/components/consult-fields";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import { highPlanPriceYen } from "@/lib/coaching-plan";
import { formatDateTime, formatYen } from "@/lib/format";
import {
  answerRows, consultMinorNote, consultNotContractNote, supportConsultStatusLabels, supportStandardContent, type SupportConsult, type SupportConsultProposal,
} from "@/lib/support-consult";
import type { CoachEvent } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useState } from "react";

// 無料相談のやりとり。選手は回答の確認・聞き直しへの答え・提案の確認と申し込み、コーチは辞退・聞き直し・提案をする。
// チャットではなく、フォーム形式の往復だけ。

function Answers({ consult }: { consult: SupportConsult }) {
  const rows = answerRows(consult.answers, coachEventLabels[consult.event as CoachEvent] ?? consult.event);
  const sections = [...new Set(rows.map((r) => r.section))];
  return (
    <details className="rounded-xl border border-white/20 bg-white/5 p-3 text-sm">
      <summary className="cursor-pointer font-semibold">{L("相談フォームの回答")}</summary>
      <div className="mt-2 space-y-3">
        {sections.map((s) => (
          <section key={s}>
            <h4 className="text-xs font-bold text-slate-300">{s}</h4>
            <dl className="space-y-1">
              {rows.filter((r) => r.section === s).map((r) => (
                <div key={r.label}><dt className="text-xs text-slate-300">{r.label}</dt><dd className="whitespace-pre-wrap break-words text-slate-100">{r.value}</dd></div>
              ))}
            </dl>
          </section>
        ))}
      </div>
    </details>
  );
}

// ProposalCard は、提案を1画面で確認できる形にする。期間・内容・価格・開始日・有効期限。
export function ProposalCard({ p, role }: { p: SupportConsultProposal; role: "athlete" | "coach" }) {
  const rows: [string, string][] = [
    [L("大会"), p.competition_name],
    [L("期間"), L("{support_start_date} 〜 {competition_date}（1ヶ月・{duration_days}日間）", { support_start_date: p.support_start_date, competition_date: p.competition_date, duration_days: p.duration_days })],
    [L("サポートの内容"), L(supportStandardContent)],
    [L("ビデオ通話"), p.call_count ? L("{call_count}回・1回{call_minutes}分", { call_count: p.call_count, call_minutes: p.call_minutes }) : L("なし")],
    [L("価格"), L("{v0}（カード払い）", { v0: formatYen(p.price_yen) })],
    [L("提案の有効期限"), L("{valid_until_date} の終わりまで（それまでに申し込みを済ませてください）", { valid_until_date: p.valid_until_date })],
  ];
  return (
    <section aria-label={L("コーチの提案")} className="space-y-2 rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm">
      <h3 className="font-bold">{L("コーチの提案")}</h3>
      <dl className="space-y-1.5">
        {rows.filter(([, v]) => v).map(([k, v]) => <div key={k}><dt className="text-xs text-slate-300">{k}</dt><dd className="text-slate-100">{v}</dd></div>)}
        {p.content_note ? <div><dt className="text-xs text-slate-300">{L("内容の補足")}</dt><dd className="whitespace-pre-wrap break-words">{p.content_note}</dd></div> : null}
        {p.answer_to_question ? <div><dt className="text-xs text-slate-300">{L("聞きたいことへの回答")}</dt><dd className="whitespace-pre-wrap break-words">{p.answer_to_question}</dd></div> : null}
      </dl>
      {role === "coach" ? <p className="text-xs text-slate-300">{L("運営手数料{fee_percent}%を引いた、あなたの受け取り: {v1}。報酬は契約終了時に経過日数の日割りで確定し、期間中は毎月末までの分を仮払いします。", { fee_percent: p.fee_percent, v1: formatYen(p.coach_receives_yen) })}</p> : null}
      <SupportTermsNotice priceYen={p.price_yen} days={p.duration_days} />
    </section>
  );
}

function useAction(token: string | null, onChange: (c: SupportConsult) => void) {
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const { pending, acquire, release } = useActionLock();
  async function run(path: string, body?: unknown, after?: (r: unknown) => void | Promise<void>) {
    if (!acquire()) return;
    setError("");
    setFields({});
    try {
      const r = await apiFetch<{ consult?: SupportConsult }>(path, { method: "POST", token, body: body === undefined ? undefined : JSON.stringify(body) });
      if (after) await after(r);
      else if (r.consult) onChange(r.consult);
    } catch (e) {
      if (e instanceof ApiRequestError && e.fields) setFields(e.fields);
      else setError(e instanceof ApiRequestError ? e.message : L("操作できませんでした"));
    } finally {
      release();
    }
  }
  return { error, fields, pending, run };
}

function ClarifyForm({ consult, token, onChange }: { consult: SupportConsult; token: string | null; onChange: (c: SupportConsult) => void }) {
  const [answer, setAnswer] = useState("");
  const { error, fields, pending, run } = useAction(token, onChange);
  const guard = useSolicitationGuard();
  return (
    <form className="space-y-2" onSubmit={(e) => { e.preventDefault(); if (guard.allow(answer)) void run(`/v1/support-consults/${consult.id}/answer`, { answer }); }}>
      <ContactNotice />
      {guard.warning}
      <FieldBlock label={L("答え（500文字まで）")} error={fields.answer}>
        <Textarea aria-label={L("聞き直しへの答え")} rows={3} maxLength={500} value={answer} onChange={(e) => setAnswer(e.target.value)} />
      </FieldBlock>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" disabled={pending || !answer.trim()}>{L("答えを送る")}</Button>
    </form>
  );
}

function AthleteAccept({ consult, token, onChange }: { consult: SupportConsult; token: string | null; onChange: (c: SupportConsult) => void }) {
  const [consents, setConsents] = useState<ContractConsents>(noConsents);
  const { error, fields, pending, run } = useAction(token, onChange);
  const p = consult.proposal!;
  if (consult.minor) {
    // 未成年本人は申し込めない。保護者に提案を送り、保護者が同意してお支払いをすると契約が成立する。
    return (
      <div className="space-y-3">
        <p className="text-sm text-slate-200">{L(consultNotContractNote)}</p>
        <p className="text-sm font-semibold text-slate-100">{L("{consultMinorNote}この提案を保護者に送ると、保護者のページで内容を確認できます。保護者が同意してカードでお支払いをすると契約が成立して始まります。あなたが送っただけでは、契約もお支払いも起きません。", { consultMinorNote: L(consultMinorNote) })}</p>
        {p.expired ? <p role="alert" className="text-sm text-red-400">{L("この提案は有効期限が過ぎています。")}</p> : null}
        {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
        <Button type="button" className="w-full" disabled={pending || p.expired} onClick={() => void run(`/v1/support-consults/${consult.id}/send-to-guardian`)}>{L("保護者に提案を送る")}</Button>
      </div>
    );
  }
  const ok = consents.agree_report && consents.agree_detection && !p.expired;
  return (
    <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); void run(`/v1/support-consults/${consult.id}/accept`, consents, async () => { const r = await apiFetch<{ consult: SupportConsult }>(`/v1/support-consults/${consult.id}`, { token }); onChange(r.consult); }); }}>
      <p className="text-sm text-slate-200">{L(consultNotContractNote)}</p>
      {p.expired ? <p role="alert" className="text-sm text-red-400">{L("この提案は有効期限が過ぎています。")}</p> : null}
      <ContractConsentChecks value={consents} onChange={setConsents} errors={fields} />
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || !ok}>{L("この提案で申し込む")}</Button>
    </form>
  );
}

const today = () => new Date().toLocaleDateString("sv-SE", { timeZone: "Asia/Tokyo" });

function ProposalForm({ consult, token, onChange }: { consult: SupportConsult; token: string | null; onChange: (c: SupportConsult) => void }) {
  const a = consult.answers;
  const [d, setD] = useState({
    competition_name: a.goal_decided ? a.competition_name : "", support_start_date: a.start_timing === "date" ? a.start_date : today(),
    call_count: 0, call_minutes: 30, price: "", ...noConsents, content_note: "", answer_to_question: "", valid_until: "",
  });
  const { error, fields, pending, run } = useAction(token, onChange);
  const price = Number(d.price.replace(/[,，円\s]/g, ""));
  const guard = useSolicitationGuard();
  const set = (patch: Partial<typeof d>) => setD((x) => ({ ...x, ...patch }));
  function submit() {
    if (!guard.allow([d.competition_name, d.content_note, d.answer_to_question].join("\n"))) return;
    let confirmHigh = false;
    if (Number.isFinite(price) && price > highPlanPriceYen) {
      if (!confirm(L("{v0}で提案します。打ち間違いではありませんか？", { v0: formatYen(price) }))) return;
      confirmHigh = true;
    }
    void run(`/v1/support-consults/${consult.id}/propose`, { ...d, price_yen: Number.isFinite(price) ? Math.trunc(price) : 0, confirm_high_price: confirmHigh });
  }
  const fee = Number.isFinite(price) && price > 0 ? price - Math.floor((price * 25) / 100) : null;
  return (
    <form className="space-y-3 rounded-xl border border-white/20 p-3" onSubmit={(e) => { e.preventDefault(); submit(); }}>
      <h3 className="font-bold">{L("提案を出す")}</h3>
      <ContactNotice />
      {guard.warning}
      <p className="text-xs text-slate-300">{L("期間は開始日から1ヶ月（30日間）です。開始日と提案の有効期限は、相談の内容をふまえてここで決めます。目標の大会は目標として添えるだけで、期間は大会の日に左右されません。")}</p>
      <FieldBlock label={L("目標の大会名")} error={fields.competition_name}><Input aria-label={L("提案の大会名")} maxLength={100} value={d.competition_name} onChange={(e) => set({ competition_name: e.target.value })} /></FieldBlock>
      <FieldBlock label={L("開始日")} error={fields.support_start_date}><Input aria-label={L("開始日")} type="date" value={d.support_start_date} onChange={(e) => set({ support_start_date: e.target.value })} /></FieldBlock>
      <div className="rounded-lg border border-white/20 bg-white/5 p-2 text-xs text-slate-200">
        <p className="font-semibold">{L("サポートの内容（標準形・変更できません）")}</p>
        <p>{L(supportStandardContent)}</p>
      </div>
      <FieldBlock label={L("ビデオ通話の回数（0〜12）")} error={fields.call_count}><Input aria-label={L("通話回数")} type="number" min={0} max={12} value={d.call_count} onChange={(e) => set({ call_count: Number(e.target.value) })} /></FieldBlock>
      {d.call_count > 0 ? <FieldBlock label={L("通話1回の長さ")} error={fields.call_minutes}>
        <select aria-label={L("通話の長さ")} className={selectClass} value={d.call_minutes} onChange={(e) => set({ call_minutes: Number(e.target.value) })}>{[15, 30, 45, 60].map((n) => <option key={n} value={n}>{L("{n}分", { n })}</option>)}</select>
      </FieldBlock> : null}
      <FieldBlock label={L("価格（円・2,000円以上）")} error={fields.price_yen}><Input aria-label={L("価格")} inputMode="numeric" value={d.price} onChange={(e) => set({ price: e.target.value })} /></FieldBlock>
      {fee != null ? <p className="text-xs text-slate-300">{L("運営手数料25%を引いた、あなたの受け取り: {v0}", { v0: formatYen(fee) })}</p> : <p className="text-xs text-slate-300">{L("運営手数料は25%です。")}</p>}
      <FieldBlock label={L("内容の補足（任意）")} error={fields.content_note}><Textarea aria-label={L("内容の補足")} rows={3} maxLength={500} value={d.content_note} onChange={(e) => set({ content_note: e.target.value })} /></FieldBlock>
      {a.question_to_coach ? <FieldBlock label={L("選手の質問への回答「{question_to_coach}」", { question_to_coach: a.question_to_coach })} error={fields.answer_to_question}><Textarea aria-label={L("質問への回答")} rows={3} maxLength={1000} value={d.answer_to_question} onChange={(e) => set({ answer_to_question: e.target.value })} /></FieldBlock> : null}
      <FieldBlock label={L("提案の有効期限（この日の終わりまで）")} error={fields.valid_until}><Input aria-label={L("提案の有効期限")} type="date" value={d.valid_until} onChange={(e) => set({ valid_until: e.target.value })} /></FieldBlock>
      <p className="text-xs text-slate-300">{L("提案を送ることが、この契約への同意になります。選手（18歳未満は保護者）が申し込んでお支払いが済むと、あなたの引き受け操作なしに契約が成立します。")}</p>
      <ContractConsentChecks value={{ agree_report: d.agree_report, agree_detection: d.agree_detection }} onChange={(v) => set(v)} errors={fields} />
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || !d.agree_report || !d.agree_detection}>{L("提案を送る")}</Button>
    </form>
  );
}

function CoachActions({ consult, token, onChange }: { consult: SupportConsult; token: string | null; onChange: (c: SupportConsult) => void }) {
  const [question, setQuestion] = useState("");
  const [asking, setAsking] = useState(false);
  const { error, fields, pending, run } = useAction(token, onChange);
  const guard = useSolicitationGuard();
  if (consult.status !== "requested" && consult.status !== "clarifying") return null;
  return (
    <div className="space-y-3">
      {consult.status === "requested" ? <ProposalForm consult={consult} token={token} onChange={onChange} /> : <p className="text-sm text-slate-300">{L("選手の答えを待っています。")}</p>}
      {consult.status === "requested" ? (asking ? (
        <form className="space-y-2" onSubmit={(e) => { e.preventDefault(); if (guard.allow(question)) void run(`/v1/support-consults/${consult.id}/ask`, { question }); }}>
          <ContactNotice />
          {guard.warning}
          <FieldBlock label={L("選手に聞き直す（フォームの回答だけでは分からないこと・300文字まで）")} error={fields.question}>
            <Textarea aria-label={L("聞き直し")} rows={3} maxLength={300} value={question} onChange={(e) => setQuestion(e.target.value)} />
          </FieldBlock>
          <Button type="submit" disabled={pending || !question.trim()}>{L("聞き直しを送る")}</Button>
        </form>
      ) : <Button type="button" variant="secondary" onClick={() => setAsking(true)}>{L("選手に聞き直す")}</Button>) : null}
      <Button type="button" variant="secondary" disabled={pending} onClick={() => { if (confirm(L("この相談を辞退しますか？ お金はかかっていません。"))) void run(`/v1/support-consults/${consult.id}/decline`); }}>{L("辞退する")}</Button>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

export function SupportConsultDetail({ consult, role, token, onChange }: { consult: SupportConsult; role: "athlete" | "coach"; token: string | null; onChange: (c: SupportConsult) => void }) {
  const { error, pending, run } = useAction(token, onChange);
  const open = consult.status === "requested" || consult.status === "clarifying" || consult.status === "proposed" || consult.status === "sent_to_guardian";
  const pendingQuestion = consult.questions?.find((q) => q.answer == null);
  return (
    <div className="space-y-3 text-sm">
      <p className="font-semibold">{supportConsultStatusLabels[consult.status]}</p>
      {consult.status === "requested" && consult.answer_by ? <p className="text-xs text-slate-300">{L("コーチの返答の目安: {v0} まで（48時間以内）", { v0: formatDateTime(consult.answer_by) })}</p> : null}
      <Answers consult={consult} />
      {(consult.questions ?? []).map((q) => (
        <div key={q.id} className="rounded-xl border border-white/20 bg-white/5 p-3">
          <p className="text-xs text-slate-300">{L("コーチからの聞き直し")}</p><p className="whitespace-pre-wrap break-words">{q.question}</p>
          {q.answer ? <><p className="mt-2 text-xs text-slate-300">{L("選手の答え")}</p><p className="whitespace-pre-wrap break-words">{q.answer}</p></> : null}
        </div>
      ))}
      {role === "athlete" && consult.status === "clarifying" && pendingQuestion ? <ClarifyForm consult={consult} token={token} onChange={onChange} /> : null}
      {consult.proposal ? <ProposalCard p={consult.proposal} role={role} /> : null}
      {role === "athlete" && consult.status === "proposed" && consult.proposal ? <AthleteAccept consult={consult} token={token} onChange={onChange} /> : null}
      {consult.status === "accepted" && consult.contract_id ? (
        <Link href={role === "athlete" ? `/coaching/personal/${consult.contract_id}` : "/coach/contracts"} className="block text-blue-300 underline">{L("申し込みの状況を見る")}</Link>
      ) : null}
      {role === "coach" ? <CoachActions consult={consult} token={token} onChange={onChange} /> : null}
      {role === "athlete" && open ? (
        <Button type="button" variant="secondary" disabled={pending} onClick={() => { if (confirm(L("この相談を取り下げますか？ お金はかかっていません。"))) void run(`/v1/support-consults/${consult.id}/withdraw`); }}>{L("相談を取り下げる")}</Button>
      ) : null}
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}
