"use client";
import { L } from "@/lib/i18n";

import { useRef, useState, type FormEvent } from "react";
import { Input } from "./ui/input";
import { Button } from "./ui/button";
import { InjuryChoices as Choices } from "./injury-choices";
import { PrivacyPolicy } from "./privacy-policy";
import { apiFetch, apiErrorText } from "@/lib/api";
import { LeaveGuardArea, useLeaveGuard } from "./leave-guard";
import { useActionLock } from "@/lib/use-action-lock";
import {
  emptyInjuryJoinAnswers,
  injuryActivities,
  injuryBodyPartLabel,
  injuryDiagnosisMaxLength,
  injuryJoinPayload,
  injuryJoinStepError,
  injuryJoinStepsFor,
  injuryPlaces,
  injuryPreChanges,
  injurySeverities,
  injuryStages,
  jstToday,
  injuryHindsights,
  toggleHindsight,
  togglePreChange,
  type InjuryGroupMember,
  type InjuryJoinAnswers,
  type InjuryJoinStep,
} from "@/lib/injury";

const questions: Record<InjuryJoinStep, string> = {
  get diagnosis() { return L("ケガの名前は?"); },
  get severity() { return L("ケガの重さは?"); },
  get sawDoctor() { return L("病院でみてもらいましたか?"); },
  get injuredOn() { return L("いつケガをしましたか?"); },
  get place() { return L("どこでケガをしましたか?"); },
  get activity() { return L("そのとき何をしていましたか?"); },
  get preChanges() { return L("ケガの前に変わったことは?"); },
  get hindsight() { return L("いま思うと、どうすればよかったですか?"); },
  get stage() { return L("いまの段階は?"); },
};

const hints: Partial<Record<InjuryJoinStep, string>> = {
  get diagnosis() { return L("診断名がわからなければ「わからない」と書いてください。"); },
  get injuredOn() { return L("覚えているいちばん近い日でも大丈夫です。"); },
  get preChanges() { return L("当てはまるものをいくつでも選べます。「とくになし」は単独で選んでください。"); },
  get hindsight() { return L("これから同じケガをしないための、ほかの人へのヒントとして集計します。当てはまるものをいくつでも選べます。「わからない・とくにない」は単独で選んでください。"); },
};

export function InjuryJoinWizard({
  part,
  token,
  onJoined,
}: {
  part: string;
  token: string | null;
  onJoined: (member: InjuryGroupMember) => void;
}) {
  const steps = injuryJoinStepsFor(part);
  const [index, setIndex] = useState(0);
  const [answers, setAnswers] = useState<InjuryJoinAnswers>(emptyInjuryJoinAnswers);
  const [error, setError] = useState("");
  const [consent, setConsent] = useState(false);
  const privacyDialog = useRef<HTMLDialogElement>(null);
  const { pending, acquire, release } = useActionLock();
  const { setDirty } = useLeaveGuard();
  const step = steps[index];
  const last = index === steps.length - 1;
  const now = new Date();
  const today = jstToday(now);

  function change(patch: Partial<InjuryJoinAnswers> | ((current: InjuryJoinAnswers) => Partial<InjuryJoinAnswers>)) {
    setDirty(true);
    setAnswers((current) => ({ ...current, ...(typeof patch === "function" ? patch(current) : patch) }));
    setError("");
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    const problem = injuryJoinStepError(step, answers, today);
    if (problem) {
      setError(problem);
      return;
    }
    if (!last) {
      setIndex(index + 1);
      return;
    }
    if (!consent) {
      setError(L("ケガの情報の取り扱いに同意してください"));
      return;
    }
    if (!acquire()) return;
    try {
      const member = await apiFetch<InjuryGroupMember>(`/v1/injury/groups/${part}/join`, {
        method: "POST",
        token,
        body: JSON.stringify(injuryJoinPayload(part, answers)),
      });
      setDirty(false);
      onJoined(member);
    } catch (e) {
      setError(apiErrorText(e, L("参加に失敗しました")));
      release();
    }
  }

  return (
    <LeaveGuardArea>
      <form onSubmit={onSubmit} className="mx-auto max-w-md space-y-5">
        <div>
          <p className="text-sm text-slate-300">{L("{v0}のグループに入る", { v0: injuryBodyPartLabel(part) })}</p>
          <div
            className="mt-2 flex gap-1.5"
            role="progressbar"
            aria-valuemin={1}
            aria-valuemax={steps.length}
            aria-valuenow={index + 1}
            aria-label={L("質問の進み具合")}
          >
            {steps.map((item, i) => (
              <span key={item} className={`h-1 flex-1 rounded-full ${i <= index ? "bg-blue-400" : "bg-white/20"}`} />
            ))}
          </div>
          <h2 className="mt-4 text-xl font-bold">{questions[step]}</h2>
          {hints[step] ? <p className="mt-1 text-sm text-slate-300">{hints[step]}</p> : null}
          <p className="mt-1 text-xs text-slate-300">
            {index + 1} / {steps.length}
          </p>
        </div>

        {step === "diagnosis" ? (
          <Input
            aria-label={L("ケガの名前")}
            maxLength={injuryDiagnosisMaxLength}
            placeholder={L("例: 脛骨の疲労骨折")}
            value={answers.diagnosis}
            onChange={(e) => change({ diagnosis: e.target.value })}
          />
        ) : null}

        {step === "severity" ? (
          <Choices
            label={L("ケガの重さ")}
            options={injurySeverities}
            selected={(value) => answers.severity === value}
            onSelect={(severity) => change({ severity })}
          />
        ) : null}

        {step === "sawDoctor" ? (
          <Choices
            label={L("病院でみてもらったか")}
            options={[
              { value: "yes", label: L("はい") },
              { value: "no", label: L("いいえ") },
            ]}
            selected={(value) => answers.sawDoctor === (value === "yes")}
            onSelect={(value) => change({ sawDoctor: value === "yes" })}
          />
        ) : null}

        {step === "injuredOn" ? (
          <input
            type="date"
            aria-label={L("ケガをした日")}
            value={answers.injuredOn}
            max={today}
            onChange={(e) => change({ injuredOn: e.target.value })}
            className="w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2.5 text-base text-slate-100 outline-none focus:border-blue-400"
          />
        ) : null}

        {step === "place" ? (
          <Choices
            label={L("ケガをした場所")}
            options={injuryPlaces}
            selected={(value) => answers.place === value}
            onSelect={(place) => change({ place })}
          />
        ) : null}

        {step === "activity" ? (
          <Choices
            label={L("そのとき何をしていたか")}
            options={injuryActivities}
            selected={(value) => answers.activity === value}
            onSelect={(activity) => change({ activity })}
          />
        ) : null}

        {step === "preChanges" ? (
          <Choices
            label={L("ケガの前に変わったこと")}
            options={injuryPreChanges}
            selected={(value) => answers.preChanges.includes(value)}
            onSelect={(value) => change((current) => ({ preChanges: togglePreChange(current.preChanges, value) }))}
          />
        ) : null}

        {step === "hindsight" ? (
          <Choices
            label={L("どうすればよかったか")}
            options={injuryHindsights}
            selected={(value) => answers.hindsight.includes(value)}
            onSelect={(value) => change((current) => ({ hindsight: toggleHindsight(current.hindsight, value) }))}
          />
        ) : null}


        {step === "stage" ? (
          <Choices
            label={L("いまの段階")}
            options={injuryStages}
            selected={(value) => answers.stage === value}
            onSelect={(stage) => change({ stage })}
          />
        ) : null}

        {last ? (
          // ケガの情報は健康に関する情報(要配慮個人情報)なので、送る前に扱いを見せて、はっきり同意をもらう(プライバシーポリシー第4条)。
          <div className="space-y-2 text-xs leading-6 text-slate-300">
            <p>{L("回答と段階の記録は、名前を出さずに人数の割合や日数に集計し、誰でも見られるページに表示します(5人に満たない項目は出しません)。")}</p>
            <div className="flex items-start gap-2 text-sm text-slate-100">
              <input
                type="checkbox"
                id="injury-privacy-consent"
                aria-labelledby="injury-privacy-consent-label"
                checked={consent}
                onChange={(e) => {
                  setConsent(e.target.checked);
                  setError("");
                }}
                className="mt-1 h-4 w-4 shrink-0"
              />
              <span id="injury-privacy-consent-label">
                <label htmlFor="injury-privacy-consent">{L("ケガの情報を健康に関する情報として、")}</label>
                <button
                  type="button"
                  aria-haspopup="dialog"
                  className="text-left text-blue-400 underline"
                  onClick={() => privacyDialog.current?.showModal()}
                >{L("プライバシーポリシー")}</button>
                <label htmlFor="injury-privacy-consent">{L("第4条のとおり扱うことに同意します。15歳以下の人は、保護者も同意しています。")}</label>
              </span>
            </div>
          </div>
        ) : null}

        {error ? <p className="text-sm text-red-400">{error}</p> : null}

        <div className="flex gap-2">
          {index > 0 ? (
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setIndex(index - 1);
                setError("");
              }}
            >{L("戻る")}</Button>
          ) : null}
          <Button type="submit" className="flex-1" disabled={pending}>
            {last ? (pending ? L("入っています") : L("グループに入る")) : L("次へ")}
          </Button>
        </div>
      </form>
      {last ? (
        <dialog
          ref={privacyDialog}
          aria-labelledby="injury-privacy-title"
          className="fixed inset-0 m-auto max-h-[85dvh] w-11/12 max-w-2xl overflow-y-auto overscroll-contain rounded-2xl border border-white/20 bg-slate-900 text-slate-100 backdrop:bg-black/60"
        >
          <div className="sticky top-0 flex justify-end border-b border-white/15 bg-slate-900 px-4 py-2">
            <Button type="button" variant="secondary" onClick={() => privacyDialog.current?.close()}>{L("回答に戻る")}</Button>
          </div>
          <div className="p-4"><PrivacyPolicy titleId="injury-privacy-title" /></div>
        </dialog>
      ) : null}
    </LeaveGuardArea>
  );
}
