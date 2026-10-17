"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { useState, type FormEvent } from "react";

// オンラインコーチングのデモ版に入るための招待コード。フォーム添削とパーソナル指導で同じコードを使う。

export function DemoCodeForm({ intro, token, onJoined }: { intro: string; token: string | null; onJoined: () => void }) {
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch("/v1/coaching/demo", { method: "POST", token, body: JSON.stringify({ code }) });
      onJoined();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.code || err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <p className="text-sm text-slate-200">{intro}</p>
      <Input
        value={code}
        onChange={(e) => setCode(e.target.value)}
        placeholder="招待コード(8文字)"
        autoCapitalize="characters"
        autoComplete="off"
        aria-label="招待コード"
      />
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || code.trim() === ""}>
        デモに参加する
      </Button>
    </form>
  );
}
