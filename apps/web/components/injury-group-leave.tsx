"use client";
import { L } from "@/lib/i18n";

import { useState } from "react";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { injuryBodyPartLabel } from "@/lib/injury";
import type { useActionLock } from "@/lib/use-action-lock";
import { Button } from "./ui/button";

export function InjuryGroupLeave({
  part,
  token,
  lock,
  onLeft,
  inStageForm = false,
}: {
  part: string;
  token: string | null;
  lock: ReturnType<typeof useActionLock>;
  onLeft: () => void;
  inStageForm?: boolean;
}) {
  const [confirming, setConfirming] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [error, setError] = useState("");

  async function leaveGroup() {
    if (!lock.acquire()) return;
    setLeaving(true);
    setError("");
    try {
      await apiFetch(`/v1/injury/groups/${part}/me`, { method: "DELETE", token });
      onLeft();
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : L("グループを抜けられませんでした"));
      setLeaving(false);
      lock.release();
    }
  }

  return (
    <>
      <button
        type="button"
        onClick={() => { setError(""); setConfirming(true); }}
        disabled={lock.pending}
        aria-expanded={confirming}
        className={`text-sm font-semibold text-slate-300 underline-offset-2 hover:underline disabled:opacity-50 ${inStageForm ? "min-h-11 justify-self-end" : confirming ? "hidden" : ""}`}
      >
        {inStageForm ? L("グループを抜ける") : L("このグループを抜ける")}
      </button>
      {confirming ? (
        <div className={`space-y-2 rounded-xl border border-white/20 glass bg-white/8 p-3 text-sm text-slate-200 ${inStageForm ? "col-span-2" : ""}`}>
          <p>{L("{v0}のグループを抜けますか?段階の記録は消えます。投稿は残ります。", { v0: injuryBodyPartLabel(part) })}</p>
          <div className="flex gap-2">
            <Button type="button" variant="danger" disabled={lock.pending} onClick={() => void leaveGroup()}>
              {leaving ? L("抜けています…") : L("抜ける")}
            </Button>
            <Button type="button" variant="secondary" disabled={lock.pending} onClick={() => setConfirming(false)}>{L("やめる")}</Button>
          </div>
          {error ? <p role="alert" className="text-red-400">{error}</p> : null}
        </div>
      ) : null}
    </>
  );
}
