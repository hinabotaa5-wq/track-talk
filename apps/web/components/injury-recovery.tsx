"use client";

import { useState } from "react";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { injuryStages, injuryUpdateMaxLength, isInjuryStage, type InjuryStage } from "@/lib/injury";
import type { InjuryUpdate } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { useActionLock } from "@/lib/use-action-lock";

const chipClass = "rounded-full border px-3.5 py-1.5 text-sm font-semibold transition active:scale-95";
const chipIdle = "border-white/20 glass bg-white/10 text-slate-200 hover:bg-white/16 hover:text-white";
const chipActive = "border-emerald-400 bg-emerald-950/60 text-white";

export function InjuryRecovery({
  postId,
  updates,
  mine,
  onChanged,
}: {
  postId: string;
  updates: InjuryUpdate[];
  mine: boolean;
  onChanged: () => Promise<void> | void;
}) {
  const { token } = useAuth();
  const lock = useActionLock();
  const [stage, setStage] = useState<InjuryStage | "">("");
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editStage, setEditStage] = useState<InjuryStage | "">("");
  const [editBody, setEditBody] = useState("");
  const [editError, setEditError] = useState("");

  if (!mine && updates.length === 0) return null;

  async function submit() {
    if (!stage) return setError("いまの状況を選んでください");
    if (!body.trim()) return setError("経過を書いてください");
    if (!lock.acquire()) return;
    try {
      await apiFetch(`/v1/posts/${postId}/injury/updates`, {
        method: "POST",
        token,
        body: JSON.stringify({ stage, body }),
      });
      setStage("");
      setBody("");
      setError("");
      await onChanged();
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "追加できませんでした");
    } finally {
      lock.release();
    }
  }

  function startEdit(u: InjuryUpdate) {
    setEditingId(u.id);
    setEditStage(isInjuryStage(u.stage) ? u.stage : "");
    setEditBody(u.body);
    setEditError("");
  }

  async function saveEdit(id: string) {
    if (!editStage) return setEditError("いまの状況を選んでください");
    if (!editBody.trim()) return setEditError("経過を書いてください");
    if (!lock.acquire()) return;
    try {
      await apiFetch(`/v1/posts/${postId}/injury/updates/${id}`, {
        method: "PATCH",
        token,
        body: JSON.stringify({ stage: editStage, body: editBody }),
      });
      setEditingId(null);
      await onChanged();
    } catch (e) {
      setEditError(e instanceof ApiRequestError ? e.message : "保存できませんでした");
    } finally {
      lock.release();
    }
  }

  async function remove(id: string) {
    if (!window.confirm("この経過報告を削除しますか?")) return;
    if (!lock.acquire()) return;
    try {
      await apiFetch(`/v1/posts/${postId}/injury/updates/${id}`, { method: "DELETE", token });
      await onChanged();
    } catch (e) {
      alert(e instanceof ApiRequestError ? e.message : "削除できませんでした");
    } finally {
      lock.release();
    }
  }

  return (
    <section className="space-y-3">
      <h2 className="text-xl font-semibold text-slate-100">復帰までの経過</h2>
      {updates.length > 0 ? (
        <ol className="space-y-3 border-l border-emerald-800/60 pl-4">
          {updates.map((u) => (
            <li key={u.id} className="space-y-1.5">
              <p className="flex flex-wrap items-center gap-2 text-xs text-slate-300">
                <span className="rounded-full bg-emerald-950/70 px-2.5 py-0.5 font-bold text-emerald-300 border border-emerald-800/60">
                  {u.stage_label}
                </span>
                <span>{formatDateTime(u.created_at)}</span>
                {mine && editingId !== u.id ? (
                  <span className="ml-auto flex gap-3">
                    <button
                      type="button"
                      disabled={lock.pending}
                      onClick={() => startEdit(u)}
                      className="text-slate-300 underline-offset-2 hover:text-white hover:underline"
                    >
                      編集
                    </button>
                    <button
                      type="button"
                      disabled={lock.pending}
                      onClick={() => remove(u.id)}
                      className="text-slate-300 underline-offset-2 hover:text-white hover:underline"
                    >
                      削除
                    </button>
                  </span>
                ) : null}
              </p>
              {editingId === u.id ? (
                <div className="space-y-2.5 rounded-xl border border-white/20 glass bg-white/10 p-3">
                  <div className="flex flex-wrap gap-2">
                    {injuryStages.map((item) => (
                      <button
                        key={item.value}
                        type="button"
                        aria-pressed={editStage === item.value}
                        onClick={() => setEditStage(item.value)}
                        className={`${chipClass} ${editStage === item.value ? chipActive : chipIdle}`}
                      >
                        {item.label}
                      </button>
                    ))}
                  </div>
                  <Textarea
                    value={editBody}
                    maxLength={injuryUpdateMaxLength}
                    rows={3}
                    onChange={(e) => setEditBody(e.target.value)}
                  />
                  {editError ? <p className="text-sm text-red-400">{editError}</p> : null}
                  <div className="flex gap-2">
                    <Button type="button" disabled={lock.pending} onClick={() => saveEdit(u.id)}>
                      保存
                    </Button>
                    <Button type="button" variant="secondary" disabled={lock.pending} onClick={() => setEditingId(null)}>
                      キャンセル
                    </Button>
                  </div>
                </div>
              ) : (
                <p className="whitespace-pre-wrap rounded-xl border border-white/20 glass bg-white/10 p-3 leading-7 text-slate-200">
                  {u.body}
                </p>
              )}
            </li>
          ))}
        </ol>
      ) : null}
      {mine ? (
        <div className="space-y-2.5 rounded-xl border border-white/20 glass bg-white/10 p-3.5">
          <p className="text-sm font-semibold text-slate-100">いまの状況を報告する</p>
          <div className="flex flex-wrap gap-2">
            {injuryStages.map((item) => (
              <button
                key={item.value}
                type="button"
                aria-pressed={stage === item.value}
                onClick={() => setStage(item.value)}
                className={`${chipClass} ${stage === item.value ? chipActive : chipIdle}`}
              >
                {item.label}
              </button>
            ))}
          </div>
          <Textarea
            value={body}
            maxLength={injuryUpdateMaxLength}
            rows={3}
            placeholder="痛みの具合、できるようになったこと、次の目標など"
            onChange={(e) => setBody(e.target.value)}
          />
          {error ? <p className="text-sm text-red-400">{error}</p> : null}
          <Button type="button" disabled={lock.pending} onClick={submit}>
            経過を追加
          </Button>
        </div>
      ) : null}
    </section>
  );
}
