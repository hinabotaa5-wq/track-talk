"use client";
import { L } from "@/lib/i18n";

import { useState, type FormEvent } from "react";
import { Button } from "./ui/button";
import { Textarea } from "./ui/textarea";
import { InjuryGroupLeave } from "./injury-group-leave";
import { InjuryChoices } from "./injury-choices";
import { LeaveGuardArea, useLeaveGuard } from "./leave-guard";
import { apiFetch, apiErrorText } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import {
  claimCelebration,
  emptyInjuryStageChangeAnswers,
  injuryActions,
  injuryStageChangeError,
  injuryStageChangePayload,
  injuryStageChangeStepError,
  injuryStageChangeSteps,
  injuryStageNoteMaxLength,
  injuryStages,
  toggleInjuryAction,
  type InjuryGroupMember,
  type InjuryStageChangeAnswers,
} from "@/lib/injury";

// 段階を変えるフォーム。グループのページとマイページの両方から使う。
// グループに入るときと同じように、新しい段階 → やったこと → 気づいたこと の順に1問ずつ聞く。
// 変えた後の自分と、初めて大会に復帰したので祝うかどうかを onChanged に渡す。
export function InjuryStageChange({
  part,
  token,
  member,
  onChanged,
  onCancel,
  onLeft,
}: {
  part: string;
  token: string | null;
  member: InjuryGroupMember;
  onChanged: (member: InjuryGroupMember, celebrate: boolean) => void;
  onCancel: () => void;
  onLeft?: () => void;
}) {
  const [index, setIndex] = useState(0);
  const [answers, setAnswers] = useState<InjuryStageChangeAnswers>(emptyInjuryStageChangeAnswers);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const lock = useActionLock();
  const { pending, acquire, release } = lock;
  const { setDirty, confirmLeave } = useLeaveGuard();
  const nextStageLabel = injuryStages.find(({ value }) => value === answers.stage)?.label;
  const noteLength = [...answers.note.trim()].length;
  const step = injuryStageChangeSteps[index];
  const last = index === injuryStageChangeSteps.length - 1;
  const questions = {
    stage: L("新しい段階は?"),
    actions: L("{stage_label}のあいだ、やったことは?", { stage_label: L(member.stage_label) }),
    note: L("気づいたこと・次の人へ一言(任意)"),
  } as const;

  function change(patch: Partial<InjuryStageChangeAnswers>) {
    setDirty(true);
    setAnswers((current) => ({ ...current, ...patch }));
    setError("");
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (pending) return;
    const stepProblem = injuryStageChangeStepError(step, answers, member.stage);
    if (stepProblem) {
      setError(stepProblem);
      return;
    }
    if (!last) {
      setIndex(index + 1);
      return;
    }
    const problem = injuryStageChangeError(answers, member.stage);
    if (problem) {
      setError(problem);
      return;
    }
    if (!acquire()) return;
    setSaving(true);
    setError("");
    try {
      const next = await apiFetch<InjuryGroupMember>(`/v1/injury/groups/${part}/me`, {
        method: "PATCH",
        token,
        body: JSON.stringify(injuryStageChangePayload(answers)),
      });
      setDirty(false);
      // 変更後の画面に切り替わるまで、処理中の表示と二重送信防止を保つ。
      onChanged(next, claimCelebration(part, next.stage));
    } catch (e) {
      setError(apiErrorText(e, L("段階を変えられませんでした")));
      setSaving(false);
      release();
    }
  }

  return (
    <LeaveGuardArea>
      <form onSubmit={onSubmit} className="space-y-5 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        <div>
          <div
            className="flex gap-1.5"
            role="progressbar"
            aria-valuemin={1}
            aria-valuemax={injuryStageChangeSteps.length}
            aria-valuenow={index + 1}
            aria-label={L("質問の進み具合")}
          >
            {injuryStageChangeSteps.map((item, i) => (
              <span key={item} className={`h-1 flex-1 rounded-full ${i <= index ? "bg-blue-400" : "bg-white/20"}`} />
            ))}
          </div>
          <div className="mt-3 grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2">
            <div>
              <h3 className="text-base font-bold">{questions[step]}</h3>
              {step === "actions" ? <p className="mt-1 text-sm text-slate-300">{L("いくつでも")}</p> : null}
              <p className="mt-1 text-xs text-slate-300">
                {index + 1} / {injuryStageChangeSteps.length}
              </p>
            </div>
            {onLeft ? (
              <InjuryGroupLeave
                part={part}
                token={token}
                lock={lock}
                onLeft={() => { setDirty(false); onLeft(); }}
                inStageForm
              />
            ) : null}
          </div>
        </div>

        {step === "stage" ? (
          <InjuryChoices
            label={L("新しい段階")}
            options={injuryStages}
            selected={(value) => answers.stage === value}
            onSelect={(stage) => change({ stage })}
            disabled={(value) => pending || value === member.stage}
            note={(value) => (value === member.stage ? L("(いま)") : "")}
          />
        ) : null}

        {step === "actions" ? (
          <InjuryChoices
            label={L("意識的にやったこと")}
            options={injuryActions}
            selected={(value) => answers.actions.includes(value)}
            onSelect={(value) => change({ actions: toggleInjuryAction(answers.actions, value) })}
            disabled={() => pending}
          />
        ) : null}

        {step === "note" ? (
          <div className="space-y-2">
            <Textarea
              aria-label={L("気づいたこと・次の人へ一言")}
              rows={3}
              maxLength={injuryStageNoteMaxLength}
              placeholder={L("例: 痛みがなくても、ジョグは1日おきにした")}
              value={answers.note}
              disabled={pending}
              onChange={(e) => change({ note: e.target.value })}
            />
            <p className="text-right text-xs text-slate-300">
              {noteLength} / {injuryStageNoteMaxLength}
            </p>
            <label className={`flex items-start gap-2 text-sm ${noteLength === 0 ? "text-slate-400" : "text-slate-200"}`}>
              <input
                type="checkbox"
                className="mt-0.5 size-4 accent-blue-500"
                checked={noteLength > 0 && answers.notePublic}
                disabled={pending || noteLength === 0}
                onChange={(e) => change({ notePublic: e.target.checked })}
              />
              <span>{L("グループの紹介ページに載せてよい")}<span className="block text-xs text-slate-300">{L("名前は出ません")}</span>
              </span>
            </label>
          </div>
        ) : null}

        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <p role="status" className={saving ? "text-sm text-slate-200" : "sr-only"}>
          {saving ? L("段階を変えています") : ""}
        </p>

        <div className="flex gap-2">
          <Button
            type="button"
            variant="secondary"
            onClick={() => {
              if (index === 0) {
                if (!confirmLeave(onCancel)) onCancel();
                return;
              }
              setIndex(index - 1);
              setError("");
            }}
            disabled={pending}
          >
            {index === 0 ? L("やめる") : L("戻る")}
          </Button>
          <Button type="submit" className="flex-1" disabled={pending}>
            {saving ? L("{nextStageLabel}に変更しています…", { nextStageLabel }) : last ? L("段階を変える") : L("次へ")}
          </Button>
        </div>
      </form>
    </LeaveGuardArea>
  );
}
