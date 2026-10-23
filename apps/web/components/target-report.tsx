"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "./auth-provider";
import { reportConfirmOptions, useConfirm } from "./confirm-dialog";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";

// ログインなしで読める場所(ケガ部の公開ページの一言、練習の評価のひとこと)の通報ボタン。
// ログインしていない人は、ログインしてから next の画面に戻って通報する。
export function TargetReport({
  targetType,
  id,
  next,
  noun,
}: {
  targetType: "injury_note" | "practice_rating";
  id: string;
  next: string;
  noun: string;
}) {
  const { session, token } = useAuth();
  const router = useRouter();
  const [reported, setReported] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const confirmDialog = useConfirm();

  async function report() {
    if (!session) {
      router.push(`/login?next=${encodeURIComponent(next)}`);
      return;
    }
    if (!(await confirmDialog(reportConfirmOptions({ title: `この${noun}を通報しますか？` })))) return;
    if (!acquire()) return;
    try {
      await apiFetch("/v1/reports", {
        method: "POST",
        token,
        body: JSON.stringify({ target_type: targetType, target_id: id, reason: "other", detail: "" }),
      });
      setReported(true);
      alert("通報を受け付けました");
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 409) {
        setReported(true);
        alert("すでに通報済みです");
      } else alert(e instanceof ApiRequestError ? e.message : "通報できませんでした");
    } finally {
      release();
    }
  }

  return (
    <button
      type="button"
      onClick={() => void report()}
      disabled={pending || reported}
      className={reported ? "text-xs font-semibold text-amber-300" : "text-xs text-slate-300 transition active:scale-[0.98] disabled:opacity-50"}
    >
      {reported ? "通報済み" : "通報"}
    </button>
  );
}
