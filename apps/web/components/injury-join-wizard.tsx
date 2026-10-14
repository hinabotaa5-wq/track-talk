"use client";

import { useState, type FormEvent } from "react";
import { Textarea } from "./ui/textarea";
import { Input } from "./ui/input";
import { Button } from "./ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import {
  emptyInjuryJoinAnswers,
  injuryBodyPartLabel,
  injuryJoinPayload,
  injuryJoinPlaceMaxLength,
  injuryJoinPlaces,
  injuryJoinSituationMaxLength,
  injuryJoinStepError,
  injuryJoinSteps,
  injuryNotice,
  injuryStages,
  localDateString,
  type InjuryGroupMember,
  type InjuryJoinAnswers,
  type InjuryJoinStep,
} from "@/lib/injury";

const questions: Record<InjuryJoinStep, string> = {
  injuredOn: "いつ痛めましたか?",
  place: "どこで痛めましたか?",
  situation: "どんな状況でしたか?",
  stage: "いまの状況は?",
};

const optionClass = "w-full rounded-xl border px-4 py-3 text-left text-base font-medium transition active:scale-[0.98]";
const optionIdle = "border-white/20 glass bg-white/10 text-slate-100 hover:bg-white/16";
const optionActive = "border-blue-400 bg-blue-950/60 text-white";

export function InjuryJoinWizard({
  part,
  token,
  onJoined,
}: {
  part: string;
  token: string | null;
  onJoined: (member: InjuryGroupMember) => void;
}) {
  const [index, setIndex] = useState(0);
  const [answers, setAnswers] = useState<InjuryJoinAnswers>(emptyInjuryJoinAnswers);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const step = injuryJoinSteps[index];
  const last = index === injuryJoinSteps.length - 1;
  const today = localDateString(new Date());

  function change(patch: Partial<InjuryJoinAnswers>) {
    setAnswers((current) => ({ ...current, ...patch }));
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
    if (!acquire()) return;
    try {
      const member = await apiFetch<InjuryGroupMember>(`/v1/injury/groups/${part}/join`, {
        method: "POST",
        token,
        body: JSON.stringify(injuryJoinPayload(answers)),
      });
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
    <form onSubmit={onSubmit} className="mx-auto max-w-md space-y-5">
      <div>
        <p className="text-sm text-slate-300">{injuryBodyPartLabel(part)}のグループに入る</p>
        <div className="mt-2 flex gap-1.5" role="progressbar" aria-valuemin={1} aria-valuemax={injuryJoinSteps.length} aria-valuenow={index + 1} aria-label="質問の進み具合">
          {injuryJoinSteps.map((item, i) => (
            <span key={item} className={`h-1 flex-1 rounded-full ${i <= index ? "bg-blue-400" : "bg-white/20"}`} />
          ))}
        </div>
        <h2 className="mt-4 text-xl font-bold">{questions[step]}</h2>
        <p className="mt-1 text-xs text-slate-300">
          {index + 1} / {injuryJoinSteps.length}
        </p>
      </div>

      {step === "injuredOn" ? (
        <Input type="date" aria-label="痛めた日" max={today} value={answers.injuredOn} onChange={(e) => change({ injuredOn: e.target.value })} />
      ) : null}

      {step === "place" ? (
        <div className="space-y-3">
          <div className="flex flex-wrap gap-2">
            {injuryJoinPlaces.map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => change({ place: item })}
                className={`rounded-full border px-3.5 py-1.5 text-sm font-semibold transition active:scale-95 ${answers.place === item ? optionActive : optionIdle}`}
              >
                {item}
              </button>
            ))}
          </div>
          <Input
            aria-label="痛めた場所"
            maxLength={injuryJoinPlaceMaxLength}
            placeholder="例: 土曜の河川敷のジョグ中"
            value={answers.place}
            onChange={(e) => change({ place: e.target.value })}
          />
        </div>
      ) : null}

      {step === "situation" ? (
        <Textarea
          aria-label="痛めたときの状況"
          rows={5}
          maxLength={injuryJoinSituationMaxLength}
          placeholder="例: 下りで着地した瞬間に、膝の外側に鋭い痛みが出た"
          value={answers.situation}
          onChange={(e) => change({ situation: e.target.value })}
        />
      ) : null}

      {step === "stage" ? (
        <div className="space-y-2" role="radiogroup" aria-label="いまの状況">
          {injuryStages.map((item) => (
            <button
              key={item.value}
              type="button"
              role="radio"
              aria-checked={answers.stage === item.value}
              onClick={() => change({ stage: item.value })}
              className={`${optionClass} ${answers.stage === item.value ? optionActive : optionIdle}`}
            >
              {item.label}
            </button>
          ))}
        </div>
      ) : null}

      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      <div className="flex gap-2">
        {index > 0 ? (
          <Button type="button" variant="secondary" onClick={() => { setIndex(index - 1); setError(""); }}>
            戻る
          </Button>
        ) : null}
        <Button type="submit" className="flex-1" disabled={pending}>
          {last ? (pending ? "入っています" : "グループに入る") : "次へ"}
        </Button>
      </div>
      {last ? <p className="text-xs text-slate-300">{injuryNotice}</p> : null}
    </form>
  );
}
