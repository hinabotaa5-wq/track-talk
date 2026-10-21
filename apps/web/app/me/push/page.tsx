"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { currentPushSubscription, pushSupported, subscribePush } from "@/lib/push";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiRequestError ? err.message : fallback;
}

// プッシュ通知の設定。端末ごとにオン・オフする。月1回の記録のお知らせと、コーチングの知らせに使う。
export default function PushSettingsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const { acquire, release, pending } = useActionLock();
  // null は確かめている途中。
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/push");
  }, [loading, session, router]);

  useEffect(() => {
    currentPushSubscription()
      .then((sub) => setEnabled(sub !== null))
      .catch(() => setEnabled(false));
  }, []);

  async function enable() {
    if (!acquire()) return;
    setMessage("");
    try {
      if (!pushSupported()) {
        setMessage("この端末のブラウザは通知に対応していません。iPhone はホーム画面に追加したアプリから設定してください。");
        return;
      }
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setMessage("通知が許可されませんでした。端末の設定から TrackTalk の通知を許可してください。");
        return;
      }
      const { public_key } = await apiFetch<{ public_key: string }>("/v1/push/public-key");
      const subscription = await subscribePush(public_key);
      await apiFetch<void>("/v1/me/push-subscriptions", { method: "POST", token, body: JSON.stringify(subscription) });
      setEnabled(true);
      setMessage("この端末で通知を受け取れるようになりました");
    } catch (err) {
      setMessage(errorMessage(err, "通知を設定できませんでした"));
    } finally {
      release();
    }
  }

  async function disable() {
    if (!acquire()) return;
    setMessage("");
    try {
      const sub = await currentPushSubscription();
      if (sub) {
        await apiFetch<void>("/v1/me/push-subscriptions", { method: "DELETE", token, body: JSON.stringify({ endpoint: sub.endpoint }) });
        await sub.unsubscribe();
      }
      setEnabled(false);
      setMessage("この端末では通知を受け取りません");
    } catch (err) {
      setMessage(errorMessage(err, "通知をオフにできませんでした"));
    } finally {
      release();
    }
  }

  async function test() {
    if (!acquire()) return;
    setMessage("");
    try {
      const { sent } = await apiFetch<{ sent: number }>("/v1/me/push-subscriptions/test", { method: "POST", token });
      setMessage(sent > 0 ? "テストの通知を送りました" : "通知を送れる端末がありません。もう一度オンにしてください。");
    } catch (err) {
      setMessage(errorMessage(err, "テストの通知を送れませんでした"));
    } finally {
      release();
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">通知</h1>
      <div className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
        <p className="text-sm text-slate-200">
          月に1回、大会の記録を残すお知らせをこの端末へ送ります。オン・オフは端末ごとです。
        </p>
        <p className="text-sm font-medium text-slate-100">
          {enabled === null ? "確かめています..." : enabled ? "この端末: オン" : "この端末: オフ"}
        </p>
        {enabled ? (
          <>
            <Button type="button" variant="ghost" className="w-full" disabled={pending} onClick={() => void test()}>
              テストの通知を送る
            </Button>
            <Button type="button" variant="secondary" className="w-full" disabled={pending} onClick={() => void disable()}>
              この端末の通知をオフにする
            </Button>
          </>
        ) : (
          <Button type="button" className="w-full" disabled={pending || enabled === null} onClick={() => void enable()}>
            この端末で通知をオンにする
          </Button>
        )}
        {message ? <p className="text-sm text-slate-300">{message}</p> : null}
      </div>
    </div>
  );
}
