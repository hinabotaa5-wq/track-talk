"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { FieldBlock, MultiChoice, SingleChoice, YesNo, selectClass } from "@/components/consult-fields";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import {
  accessChoices, budgetChoices, consultMinorNote, consultNotContractNote, emptyAnswers, filmChoices, focusChoices, formChoices,
  goalChoices, injuryPartChoices, injuryPastChoices, menuByChoices, pbWhenChoices, practiceDaysChoices, sessionMinutesChoices,
  startTimingChoices, timingHintChoices, videoReplyChoices, type PublicSupportConsult, type SupportConsult, type SupportConsultAnswers,
} from "@/lib/support-consult";
import type { CoachEvent } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useState } from "react";

// 大会までプランの無料相談フォーム。コーチが知りたいことを先に質問にしてあり、選ぶだけで答えられる。
// 自由記述は、大会名・一番の悩み・怪我の状態・コーチに聞きたいことだけ。支払いもカード情報の入力もない。
export function SupportConsultForm({
  coachID, consult, token, minor, onSent,
}: {
  coachID: string;
  consult: PublicSupportConsult;
  token: string | null;
  minor: boolean | null;
  onSent: (c: SupportConsult) => void;
}) {
  const [a, setA] = useState<SupportConsultAnswers>(() => ({ ...emptyAnswers(), event: consult.events.length === 1 ? consult.events[0] : "" }));
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const set = <K extends keyof SupportConsultAnswers>(key: K, value: SupportConsultAnswers[K]) => setA((x) => ({ ...x, [key]: value }));
  const err = (key: string) => fields[key];
  const paused = consult.reception.status === "paused";

  async function submit() {
    if (!acquire()) return;
    setError("");
    setFields({});
    try {
      const d = await apiFetch<{ consult: SupportConsult }>("/v1/support-consults", {
        method: "POST", token, body: JSON.stringify({ coach_id: coachID, answers: a }),
      });
      onSent(d.consult);
    } catch (e) {
      if (e instanceof ApiRequestError && e.fields) setFields(e.fields);
      else setError(e instanceof ApiRequestError ? e.message : "送れませんでした");
    } finally {
      release();
    }
  }

  const section = (title: string, children: React.ReactNode) => (
    <fieldset className="space-y-4 rounded-xl border border-white/20 p-3"><legend className="px-1 text-sm font-bold">{title}</legend>{children}</fieldset>
  );

  return (
    <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); void submit(); }}>
      <p className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm text-slate-100">
        {consultNotContractNote}
        {minor ? <span className="mt-1 block font-semibold">{consultMinorNote}</span> : null}
      </p>
      {paused ? <p role="status" className="text-sm text-amber-300">このコーチはいま新しい相談を受け付けていません。</p> : null}

      {section("A. 目標の大会", <>
        <YesNo label="目標の大会は決まっていますか" value={a.goal_decided} onChange={(v) => set("goal_decided", v)} error={err("goal_decided")} />
        {a.goal_decided === true ? <>
          <FieldBlock label="大会名" error={err("competition_name")}>
            <Input aria-label="大会名" maxLength={100} value={a.competition_name} onChange={(e) => set("competition_name", e.target.value)} />
          </FieldBlock>
          <FieldBlock label="開催日" error={err("competition_date")}>
            <Input aria-label="開催日" type="date" value={a.competition_date} onChange={(e) => set("competition_date", e.target.value)} />
          </FieldBlock>
        </> : null}
        {a.goal_decided === false ? <SingleChoice label="大会の時期の目安" choices={timingHintChoices} value={a.timing_hint} onChange={(v) => set("timing_hint", v)} error={err("timing_hint")} /> : null}
        <FieldBlock label="種目" error={err("event")}>
          <select aria-label="種目" className={selectClass} value={a.event} onChange={(e) => set("event", e.target.value)}>
            <option value="">選んでください</option>
            {consult.events.map((ev: CoachEvent) => <option key={ev} value={ev}>{coachEventLabels[ev]}</option>)}
          </select>
        </FieldBlock>
        <MultiChoice label="大会での目標" choices={goalChoices} values={a.goals} onChange={(v) => set("goals", v)} error={err("goals")} />
        <FieldBlock label="目標記録（任意・数値。例: 11.50、4:05.20）" error={err("target_record")}>
          <Input aria-label="目標記録" inputMode="decimal" maxLength={12} value={a.target_record} onChange={(e) => set("target_record", e.target.value)} />
        </FieldBlock>
      </>)}

      {section("B. いまの実力", <>
        <FieldBlock label="自己ベスト（数値。例: 12.30、4:05.20）" error={err("personal_best")}>
          <Input aria-label="自己ベスト" inputMode="decimal" maxLength={12} value={a.personal_best} onChange={(e) => set("personal_best", e.target.value)} />
        </FieldBlock>
        <SingleChoice label="いつの記録ですか" choices={pbWhenChoices} value={a.pb_when} onChange={(v) => set("pb_when", v)} error={err("pb_when")} />
        <SingleChoice label="最近の調子" choices={formChoices} value={a.form} onChange={(v) => set("form", v)} error={err("form")} />
      </>)}

      {section("C. 練習環境", <>
        <YesNo label="部活に所属していますか" value={a.club} onChange={(v) => set("club", v)} error={err("club")} />
        <YesNo label="指導してくれる人はいますか" value={a.has_coach} onChange={(v) => set("has_coach", v)} error={err("has_coach")} />
        <SingleChoice label="練習メニューは誰が決めていますか" choices={menuByChoices} value={a.menu_by} onChange={(v) => set("menu_by", v)} error={err("menu_by")} />
        <FieldBlock label="週の練習日数" error={err("practice_days")}>
          <select aria-label="週の練習日数" className={selectClass} value={a.practice_days || ""} onChange={(e) => set("practice_days", Number(e.target.value))}>
            <option value="">選んでください</option>
            {practiceDaysChoices.map((n) => <option key={n} value={n}>{n}日</option>)}
          </select>
        </FieldBlock>
        <SingleChoice label="1回の練習時間" choices={sessionMinutesChoices} value={a.session_minutes ? String(a.session_minutes) : ""} onChange={(v) => set("session_minutes", Number(v))} error={err("session_minutes")} />
        <SingleChoice label="競技場を使えますか" choices={accessChoices} value={a.track_access} onChange={(v) => set("track_access", v)} error={err("track_access")} />
        <SingleChoice label="ウエイト器具を使えますか" choices={accessChoices} value={a.weights_access} onChange={(v) => set("weights_access", v)} error={err("weights_access")} />
        <SingleChoice label="練習の動画を撮れますか" choices={filmChoices} value={a.can_film} onChange={(v) => set("can_film", v)} error={err("can_film")} />
      </>)}

      {section("D. 体と制約", <>
        <YesNo label="今、痛みや怪我はありますか" value={a.injury_now} onChange={(v) => set("injury_now", v)} error={err("injury_now")} />
        {a.injury_now === true ? <>
          <MultiChoice label="痛めている部位" choices={injuryPartChoices} values={a.injury_parts} onChange={(v) => set("injury_parts", v)} error={err("injury_parts")} />
          <FieldBlock label="怪我の状態（記入）" error={err("injury_note")}>
            <Textarea aria-label="怪我の状態" maxLength={200} rows={2} value={a.injury_note} onChange={(e) => set("injury_note", e.target.value)} />
          </FieldBlock>
        </> : null}
        <SingleChoice label="過去1年の大きな怪我" choices={injuryPastChoices} value={a.injury_past_year} onChange={(v) => set("injury_past_year", v)} error={err("injury_past_year")} />
        <YesNo label="大会までに休む・練習を減らす時期（テスト期間など）はありますか" value={a.rest_period} onChange={(v) => set("rest_period", v)} error={err("rest_period")} />
      </>)}

      {section("E. 直したいこと", <>
        <MultiChoice label="直したい場面・要素" choices={focusChoices} values={a.focus} onChange={(v) => set("focus", v)} error={err("focus")} />
        <FieldBlock label="一番の悩み（記入）" error={err("main_worry")}>
          <Textarea aria-label="一番の悩み" maxLength={500} rows={4} value={a.main_worry} onChange={(e) => set("main_worry", e.target.value)} />
        </FieldBlock>
        <YesNo label="フォームの動画はありますか（契約後に送れます）" value={a.has_video} onChange={(v) => set("has_video", v)} error={err("has_video")} />
      </>)}

      {section("F. 希望するサポート", <>
        <YesNo label="練習メニューを作ってほしいですか" value={a.want_menu} onChange={(v) => set("want_menu", v)} error={err("want_menu")} />
        <SingleChoice label="動画への返答" choices={videoReplyChoices} value={a.want_video_reply} onChange={(v) => set("want_video_reply", v)} error={err("want_video_reply")} />
        <YesNo label="大会直前の調整を見てほしいですか" value={a.want_peak} onChange={(v) => set("want_peak", v)} error={err("want_peak")} />
        <SingleChoice label="開始時期" choices={startTimingChoices} value={a.start_timing} onChange={(v) => set("start_timing", v)} error={err("start_timing")} />
        {a.start_timing === "date" ? (
          <FieldBlock label="開始したい日" error={err("start_date")}>
            <Input aria-label="開始したい日" type="date" value={a.start_date} onChange={(e) => set("start_date", e.target.value)} />
          </FieldBlock>
        ) : null}
        <SingleChoice label="予算の目安（任意）" choices={budgetChoices} value={a.budget} onChange={(v) => set("budget", a.budget === v ? "" : v)} error={err("budget")} />
        <FieldBlock label="その他、コーチに聞きたいこと（任意・記入）" error={err("question_to_coach")}>
          <Textarea aria-label="コーチに聞きたいこと" maxLength={500} rows={3} value={a.question_to_coach} onChange={(e) => set("question_to_coach", e.target.value)} />
        </FieldBlock>
      </>)}

      {Object.keys(fields).length > 0 ? <p role="alert" className="text-sm text-red-400">入力を確かめてください。</p> : null}
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" disabled={pending || paused} className="w-full">{pending ? "送っています..." : "無料で相談を送る"}</Button>
    </form>
  );
}
