"use client";

import { useState, type FormEvent } from "react";
import { Button } from "./ui/button";
import { Textarea } from "./ui/textarea";
import { InjuryChoices } from "./injury-choices";
import { LeaveGuardArea, useLeaveGuard } from "./leave-guard";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import {
  claimCelebration,
  emptyInjuryStageChangeAnswers,
  injuryActions,
  injuryStageChangeError,
  injuryStageChangePayload,
  injuryStageNoteMaxLength,
  injuryStages,
  toggleInjuryAction,
  type InjuryGroupMember,
  type InjuryStageChangeAnswers,
} from "@/lib/injury";

// 段階を変えるフォーム。グループのページとマイページの両方から使う。
// 変えた後の自分と、初めて大会に復帰したので祝うかどうかを onChanged に渡す。
export function InjuryStageChange({
  part,
  token,
  member,
  onChanged,
  onCancel,
}: {
  part: string;
  token: string | null;
  member: InjuryGroupMember;
  onChanged: (member: InjuryGroupMember, celebrate: boolean) => void;
  onCancel: () => void;
}) {
  const [answers, setAnswers] = useState<InjuryStageChangeAnswers>(emptyInjuryStageChangeAnswers);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const { setDirty } = useLeaveGuard();
  const noteLength = [...answers.note.trim()].length;

  function change(patch: Partial<InjuryStageChangeAnswers>) {
    setAnswers((current) => ({ ...current, ...patch }));
    setError("");
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    const problem = injuryStageChangeError(answers, member.stage);
    if (problem) {
      setError(problem);
      return;
    }
    if (!acquire()) return;
    try {
      const next = await apiFetch<InjuryGroupMember>(`/v1/injury/groups/${part}/me`, {
        method: "PATCH",
        token,
        body: JSON.stringify(injuryStageChangePayload(answers)),
      });
      setDirty(false);
      onChanged(next, claimCelebration(part, next.stage));
    } catch (e) {
      if (e instanceof ApiRequestError) {
        setError((e.fields ? Object.values(e.fields)[0] : "") || e.message);
      } else {
        setError("段階を変えられませんでした");
      }
    } finally {
      release();
    }
  }

  return (
    <LeaveGuardArea>
      <form onSubmit={onSubmit} className="space-y-5 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        <section className="space-y-2">
          <h3 className="text-base font-bold">新しい段階は?</h3>
          <InjuryChoices
            label="新しい段階"
            options={injuryStages}
            selected={(value) => answers.stage === value}
            onSelect={(stage) => change({ stage })}
            disabled={(value) => value === member.stage}
            note={(value) => (value === member.stage ? "(いま)" : "")}
          />
        </section>

        <section className="space-y-2">
          <h3 className="text-base font-bold">{member.stage_label}のあいだ、意識的にやったことは?</h3>
          <p className="text-sm text-slate-300">いくつでも選べます。次の人のヒントになります。</p>
          <InjuryChoices
            label="意識的にやったこと"
            options={injuryActions}
            selected={(value) => answers.actions.includes(value)}
            onSelect={(value) => change({ actions: toggleInjuryAction(answers.actions, value) })}
          />
        </section>

        <section className="space-y-2">
          <h3 className="text-base font-bold">気づいたこと・次の人へ一言(任意)</h3>
          <Textarea
            aria-label="気づいたこと・次の人へ一言"
            rows={3}
            maxLength={injuryStageNoteMaxLength}
            placeholder="例: 痛みがなくても、ジョグは1日おきにした"
            value={answers.note}
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
              disabled={noteLength === 0}
              onChange={(e) => change({ notePublic: e.target.checked })}
            />
            <span>
              グループの紹介ページに載せてよい
              <span className="block text-xs text-slate-300">ニックネームは出ず、段階と種目だけが表示されます。</span>
            </span>
          </label>
        </section>

        {error ? <p className="text-sm text-red-400">{error}</p> : null}

        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCancel} disabled={pending}>
            やめる
          </Button>
          <Button type="submit" className="flex-1" disabled={pending}>
            {pending ? "変えています" : "段階を変える"}
          </Button>
        </div>
      </form>
    </LeaveGuardArea>
  );
}
