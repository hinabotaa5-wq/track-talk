"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { currentPushSubscription, pushSupported, subscribePush } from "@/lib/push";
import type { NotificationCategory, NotificationSetting } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiRequestError ? err.message : fallback;
}

const categoryLabels: Record<NotificationCategory, { title: string; detail: string }> = {
  comment: { title: "コメント・返信", detail: "自分の投稿へのコメントと、自分のコメントへの返信" },
  practice: { title: "練習の参加", detail: "参加申請、承認、解散したあとの評価のお願い" },
  recruitment: { title: "新しい募集", detail: "ほかの人が練習の募集を投稿したとき" },
  injury: { title: "ケガの経過報告", detail: "「わかる」か保存をしたケガの投稿に経過報告があったとき" },
  review: { title: "フォーム添削", detail: "返答、補足、期限切れ、「役に立ちましたか」" },
  coaching: { title: "パーソナル指導・コーチ", detail: "申し込み、承諾、開始、終了、メッセージ、コーチの応募の結果" },
  monthly: { title: "月1回の記録のお知らせ", detail: "毎月1日に、大会の記録を残すお知らせ" },
  announcement: { title: "運営からのお知らせ", detail: "新しい機能が使えるようになったときなど、運営から全員へのお知らせ" },
};

function Switch({ checked, label, disabled, onChange }: { checked: boolean; label: string; disabled: boolean; onChange: (next: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative h-7 w-12 shrink-0 rounded-full transition disabled:opacity-50 ${checked ? "bg-blue-600" : "bg-white/25"}`}
    >
      <span className={`absolute top-1 size-5 rounded-full bg-white transition-all ${checked ? "left-6" : "left-1"}`} />
    </button>
  );
}

// 通知の種類ごとに、ベル(アプリ内)とプッシュを切り替える。設定はアカウントごとで、どの端末にも効く。
function NotificationSettings({ token }: { token: string | null }) {
  const [settings, setSettings] = useState<NotificationSetting[] | null>(null);
  const [saving, setSaving] = useState<NotificationCategory | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    apiFetch<{ settings: NotificationSetting[] }>("/v1/me/notification-settings", { token })
      .then((data) => setSettings(data.settings))
      .catch(() => setError("通知の設定を読み込めませんでした"));
  }, [token]);

  async function update(next: NotificationSetting) {
    if (!token || saving) return;
    const before = settings;
    setSaving(next.category);
    setError("");
    setSettings((current) => current?.map((item) => (item.category === next.category ? next : item)) ?? current);
    try {
      await apiFetch<NotificationSetting>(`/v1/me/notification-settings/${next.category}`, {
        method: "PUT",
        token,
        body: JSON.stringify({ in_app: next.in_app, push: next.push }),
      });
    } catch (err) {
      setSettings(before);
      setError(errorMessage(err, "設定を保存できませんでした"));
    } finally {
      setSaving(null);
    }
  }

  return (
    <section aria-labelledby="notification-kinds" className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
      <h2 id="notification-kinds" className="text-base font-semibold text-slate-100">
        通知の種類
      </h2>
      <p className="text-sm text-slate-200">
        種類ごとに、アプリ内(ベルの一覧)とプッシュ(この画面でオンにした端末)を選べます。
      </p>
      {settings === null && !error ? <p className="text-sm text-slate-300">読み込み中...</p> : null}
      {settings ? (
        <ul className="divide-y divide-white/15">
          <li className="flex items-center gap-3 pb-2 text-xs text-slate-300" aria-hidden>
            <span className="flex-1" />
            <span className="w-12 text-center">アプリ内</span>
            <span className="w-12 text-center">プッシュ</span>
          </li>
          {settings.map((item) => {
            const label = categoryLabels[item.category] ?? { title: item.category, detail: "" };
            return (
              <li key={item.category} className="flex items-center gap-3 py-3">
                <div className="flex-1">
                  <p className="text-sm font-medium text-slate-100">{label.title}</p>
                  <p className="text-xs text-slate-300">{label.detail}</p>
                </div>
                <Switch
                  checked={item.in_app}
                  label={`${label.title}: アプリ内`}
                  disabled={saving !== null}
                  onChange={(next) => void update({ ...item, in_app: next })}
                />
                <Switch
                  checked={item.push}
                  label={`${label.title}: プッシュ`}
                  disabled={saving !== null}
                  onChange={(next) => void update({ ...item, push: next })}
                />
              </li>
            );
          })}
        </ul>
      ) : null}
      <p className="text-xs text-slate-300">
        お支払い・返金・送金、保護者の同意、運営からの連絡、受付中のコーチへの添削の依頼は、大事な知らせなのでオフにできません。
      </p>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}

// 通知の設定。プッシュは端末ごとにオン・オフし、届く種類はアカウントで選ぶ。
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
        <h2 className="text-base font-semibold text-slate-100">この端末のプッシュ通知</h2>
        <p className="text-sm text-slate-200">
          オンにすると、下で選んだ種類の通知をこの端末へ送ります。オン・オフは端末ごとです。
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
      <NotificationSettings token={token} />
    </div>
  );
}
