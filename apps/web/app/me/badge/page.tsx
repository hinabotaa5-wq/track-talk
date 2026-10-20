"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

export default function BadgePage() {
  const { session, token, profile, loading, refreshProfile } = useAuth();
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const shown = profile?.post_master_badge_public ?? true;

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/badge");
  }, [loading, session, router]);

  async function toggle(next: boolean) {
    if (!token) return;
    setSaving(true);
    setMessage("");
    setError("");
    try {
      await apiFetch("/v1/me/badge", {
        method: "PATCH",
        token,
        body: JSON.stringify({ post_master_badge_public: next }),
      });
      await refreshProfile();
      setMessage(next ? "バッジを表示します。" : "バッジを隠しました。");
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "更新に失敗しました。");
    } finally {
      setSaving(false);
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">バッジ</h1>
      <div className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-4">
        <label className="flex items-center gap-2 text-sm text-slate-200">
          <input
            type="checkbox"
            className="size-4 accent-blue-500"
            checked={shown}
            disabled={saving}
            onChange={(e) => void toggle(e.target.checked)}
          />
          「投稿マスター」のバッジを表示する
        </label>
        <p className="text-xs text-slate-400">
          いいねがついた投稿が30件以上になると、投稿の詳細でニックネームの横に出ます。ニックネームを公開していないときは出ません。
        </p>
      </div>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      {message ? <p className="text-sm text-emerald-400">{message}</p> : null}
    </div>
  );
}
