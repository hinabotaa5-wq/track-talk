"use client";

import { useState, type FormEvent } from "react";
import { Input } from "./ui/input";
import { Button } from "./ui/button";
import { InjuryChoices as Choices } from "./injury-choices";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { LeaveGuardArea, useLeaveGuard } from "./leave-guard";
import { useActionLock } from "@/lib/use-action-lock";
import {
  emptyInjuryJoinAnswers,
  injuryActivities,
  injuryBodyPartLabel,
  injuryDiagnosisMaxLength,
  injuryEvents,
  injuryJoinPayload,
  injuryJoinStepError,
  injuryJoinStepsFor,
  injuryPlaces,
  injuryPreChanges,
  injurySeverities,
  injuryStages,
  localMonthString,
  recentMonths,
  togglePreChange,
  type InjuryGroupMember,
  type InjuryJoinAnswers,
  type InjuryJoinStep,
} from "@/lib/injury";

const questions: Record<InjuryJoinStep, string> = {
  diagnosis: "ケガの名前は?",
  severity: "ケガの重さは?",
  sawDoctor: "病院でみてもらいましたか?",
  injuredMonth: "いつごろケガをしましたか?",
  place: "どこでケガをしましたか?",
  activity: "そのとき何をしていましたか?",
  preChanges: "ケガの前に変わったことは?",
  event: "種目は?",
  stage: "いまの段階は?",
};

const hints: Partial<Record<InjuryJoinStep, string>> = {
  diagnosis: "診断名がわからなければ「わからない」と書いてください。",
  injuredMonth: "だいたいで大丈夫です。",
  preChanges: "当てはまるものをいくつでも選べます。",
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
  const { pending, acquire, release } = useActionLock();
  const { setDirty } = useLeaveGuard();
  const step = steps[index];
  const last = index === steps.length - 1;
  const now = new Date();
  const thisMonth = localMonthString(now);

  function change(patch: Partial<InjuryJoinAnswers>) {
    setAnswers((current) => ({ ...current, ...patch }));
    setError("");
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    const problem = injuryJoinStepError(step, answers, thisMonth);
    if (problem) {
      setError(problem);
      return;
    }
    if (!last) {
      setIndex(index + 1);
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
      if (e instanceof ApiRequestError) {
        setError((e.fields ? Object.values(e.fields)[0] : "") || e.message);
      } else {
        setError("参加に失敗しました");
      }
      release();
    }
  }

  return (
    <LeaveGuardArea>
      <form onSubmit={onSubmit} className="mx-auto max-w-md space-y-5">
        <div>
          <p className="text-sm text-slate-300">{injuryBodyPartLabel(part)}のグループに入る</p>
          <div
            className="mt-2 flex gap-1.5"
            role="progressbar"
            aria-valuemin={1}
            aria-valuemax={steps.length}
            aria-valuenow={index + 1}
            aria-label="質問の進み具合"
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
            aria-label="ケガの名前"
            maxLength={injuryDiagnosisMaxLength}
            placeholder="例: 脛骨の疲労骨折"
            value={answers.diagnosis}
            onChange={(e) => change({ diagnosis: e.target.value })}
          />
        ) : null}

        {step === "severity" ? (
          <Choices
            label="ケガの重さ"
            options={injurySeverities}
            selected={(value) => answers.severity === value}
            onSelect={(severity) => change({ severity })}
          />
        ) : null}

        {step === "sawDoctor" ? (
          <Choices
            label="病院でみてもらったか"
            options={[
              { value: "yes", label: "はい" },
              { value: "no", label: "いいえ" },
            ]}
            selected={(value) => answers.sawDoctor === (value === "yes")}
            onSelect={(value) => change({ sawDoctor: value === "yes" })}
          />
        ) : null}

        {step === "injuredMonth" ? (
          <select
            aria-label="ケガをした月"
            value={answers.injuredMonth}
            onChange={(e) => change({ injuredMonth: e.target.value })}
            className="w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2.5 text-base text-slate-100 outline-none focus:border-blue-400"
          >
            <option value="" className="text-slate-900">
              月を選ぶ
            </option>
            {recentMonths(now).map((month) => (
              <option key={month.value} value={month.value} className="text-slate-900">
                {month.label}
              </option>
            ))}
          </select>
        ) : null}

        {step === "place" ? (
          <Choices
            label="ケガをした場所"
            options={injuryPlaces}
            selected={(value) => answers.place === value}
            onSelect={(place) => change({ place })}
          />
        ) : null}

        {step === "activity" ? (
          <Choices
            label="そのとき何をしていたか"
            options={injuryActivities}
            selected={(value) => answers.activity === value}
            onSelect={(activity) => change({ activity })}
          />
        ) : null}

        {step === "preChanges" ? (
          <Choices
            label="ケガの前に変わったこと"
            options={injuryPreChanges}
            selected={(value) => answers.preChanges.includes(value)}
            onSelect={(value) => change({ preChanges: togglePreChange(answers.preChanges, value) })}
          />
        ) : null}

        {step === "event" ? (
          <Choices
            label="種目"
            options={injuryEvents}
            selected={(value) => answers.event === value}
            onSelect={(value) => change({ event: value })}
          />
        ) : null}

        {step === "stage" ? (
          <Choices
            label="いまの段階"
            options={injuryStages}
            selected={(value) => answers.stage === value}
            onSelect={(stage) => change({ stage })}
          />
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
            >
              戻る
            </Button>
          ) : null}
          <Button type="submit" className="flex-1" disabled={pending}>
            {last ? (pending ? "入っています" : "グループに入る") : "次へ"}
          </Button>
        </div>
      </form>
    </LeaveGuardArea>
  );
}
