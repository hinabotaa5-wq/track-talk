"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { useAuth } from "./auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";

// 公開ページの一言の通報ボタン。ログインしていない人はログインしてから通報する。
export function InjuryNoteReport({ id, part }: { id: string; part: string }) {
  const { session, token } = useAuth();
  const router = useRouter();
  const [reported, setReported] = useState(false);
  const { pending, acquire, release } = useActionLock();

  async function report() {
    if (!session) {
      router.push(`/login?next=${encodeURIComponent(`/injury/${part}`)}`);
      return;
    }
    if (!confirm("この一言を通報しますか？")) return;
    if (!acquire()) return;
    try {
      await apiFetch("/v1/reports", {
        method: "POST",
        token,
        body: JSON.stringify({ target_type: "injury_note", target_id: id, reason: "other", detail: "" }),
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
      className={reported ? "text-xs font-semibold text-amber-300" : "text-xs text-slate-300 transition active:scale-95 disabled:opacity-50"}
    >
      {reported ? "通報済み" : "通報"}
    </button>
  );
}
