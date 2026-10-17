"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatYen, pushSupported, subscribePush } from "@/lib/push";
import type { CoachingGuardian, CoachingMe } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";

// フォーム添削を始める前の準備（生まれた年月、保護者の同意、通知）。
// 添削の画面ができるまではメニューから辿れない。

const guardianLabels: Record<CoachingGuardian["status"], string> = {
  pending: "保護者の同意待ち",
  consented: "保護者が同意済み",
  declined: "保護者が同意しませんでした",
  revoked: "保護者が同意を取り消しました",
};

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiRequestError ? err.message : fallback;
}

export default function CoachingSetupPage() {
  const { session, token, loading } = useAuth();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const [month, setMonth] = useState("");
  const [monthError, setMonthError] = useState("");
  const [link, setLink] = useState("");
  const [linkMessage, setLinkMessage] = useState("");
  const [pushMessage, setPushMessage] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((data) => {
        if (!cancelled) setMe(data);
      })
      .catch((err) => {
        if (!cancelled) setLoadError(errorMessage(err, "読み込みに失敗しました"));
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function onBirthMonth(event: FormEvent) {
    event.preventDefault();
    if (!month || !acquire()) return;
    setMonthError("");
    try {
      setMe(
        await apiFetch<CoachingMe>("/v1/me/birth-month", {
          method: "PUT",
          token,
          body: JSON.stringify({ birth_month: month }),
        }),
      );
    } catch (err) {
      setMonthError(err instanceof ApiRequestError ? err.fields?.birth_month || err.message : "登録に失敗しました");
    } finally {
      release();
    }
  }

  async function onIssueLink() {
    if (!acquire()) return;
    setLinkMessage("");
    try {
      const result = await apiFetch<CoachingGuardian & { token: string }>("/v1/me/guardian-link", {
        method: "POST",
        token,
      });
      const { token: guardianToken, ...guardian } = result;
      setLink(`${window.location.origin}/guardian/${guardianToken}`);
      setMe((prev) => (prev ? { ...prev, guardian } : prev));
    } catch (err) {
      setLinkMessage(errorMessage(err, "リンクを作れませんでした"));
    } finally {
      release();
    }
  }

  async function onShare() {
    const text = "TrackTalk のフォーム添削を使いたいので、保護者の同意をお願いします。";
    try {
      if (navigator.share) {
        await navigator.share({ title: "保護者の同意のお願い", text, url: link });
        return;
      }
      await navigator.clipboard.writeText(link);
      setLinkMessage("リンクをコピーしました");
    } catch {
      // 共有を閉じたときは何もしない
    }
  }

  async function onEnablePush() {
    if (!acquire()) return;
    setPushMessage("");
    try {
      if (!pushSupported()) {
        setPushMessage("この端末のブラウザは通知に対応していません。iPhone はホーム画面に追加したアプリから設定してください。");
        return;
      }
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setPushMessage("通知が許可されませんでした。端末の設定から TrackTalk の通知を許可してください。");
        return;
      }
      const { public_key } = await apiFetch<{ public_key: string }>("/v1/push/public-key");
      const subscription = await subscribePush(public_key);
      await apiFetch<void>("/v1/me/push-subscriptions", {
        method: "POST",
        token,
        body: JSON.stringify(subscription),
      });
      setMe(await apiFetch<CoachingMe>("/v1/me/coaching", { token }));
      setPushMessage("この端末で通知を受け取れるようになりました");
    } catch (err) {
      setPushMessage(errorMessage(err, "通知を設定できませんでした"));
    } finally {
      release();
    }
  }

  async function onTestPush() {
    if (!acquire()) return;
    setPushMessage("");
    try {
      const { sent } = await apiFetch<{ sent: number }>("/v1/me/push-subscriptions/test", { method: "POST", token });
      setPushMessage(sent > 0 ? "テストの通知を送りました" : "通知を送れる端末がありません。もう一度設定してください。");
    } catch (err) {
      setPushMessage(errorMessage(err, "テストの通知を送れませんでした"));
    } finally {
      release();
    }
  }

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }
  if (loadError) {
    return <p className="text-sm text-red-400">{loadError}</p>;
  }
  if (!me) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-8">
      <h1 className="text-2xl font-bold">フォーム添削の準備</h1>

      <section className="space-y-3">
        <h2 className="font-bold">生まれた年月</h2>
        {me.birth_month ? (
          <p className="text-sm text-slate-200">
            {me.birth_month.replace("-", "年")}月（{me.is_minor ? "18歳未満" : "18歳以上"}）。変更はお問い合わせからお願いします。
          </p>
        ) : (
          <form onSubmit={onBirthMonth} className="space-y-2">
            <p className="text-sm text-slate-300">
              18歳未満の人は、保護者の同意が必要です。年齢の確認のために使い、他の人には見えません。登録したあとは自分では変えられません。
            </p>
            <Input type="month" value={month} onChange={(e) => setMonth(e.target.value)} aria-label="生まれた年月" />
            {monthError ? <p className="text-sm text-red-400">{monthError}</p> : null}
            <Button type="submit" className="w-full" disabled={!month || pending}>
              登録する
            </Button>
          </form>
        )}
      </section>

      {me.is_minor ? (
        <section className="space-y-3">
          <h2 className="font-bold">保護者の同意</h2>
          <p className="text-sm text-slate-200">
            {me.guardian ? guardianLabels[me.guardian.status] : "まだ保護者にリンクを送っていません"}
            {me.guardian?.status === "consented" ? `（${formatYen(me.guardian.monthly_limit_yen)}）` : null}
          </p>
          {link ? (
            <div className="space-y-2">
              <Input readOnly value={link} onFocus={(e) => e.target.select()} aria-label="保護者に送るリンク" />
              <Button type="button" className="w-full" onClick={() => void onShare()}>
                LINE などで保護者に送る
              </Button>
            </div>
          ) : (
            <Button type="button" variant="secondary" className="w-full" disabled={pending} onClick={() => void onIssueLink()}>
              {me.guardian ? "保護者に送るリンクを作り直す" : "保護者に送るリンクを作る"}
            </Button>
          )}
          {me.guardian && !link ? (
            <p className="text-xs text-slate-400">作り直すと、前に送ったリンクは使えなくなります。</p>
          ) : null}
          {linkMessage ? <p className="text-sm text-slate-300">{linkMessage}</p> : null}
        </section>
      ) : null}

      <section className="space-y-3">
        <h2 className="font-bold">通知</h2>
        <p className="text-sm text-slate-300">
          添削が返ってきたときや、パーソナル指導の申し込みに返事が来たときに、この端末へ通知します。通知を受け取れる端末: {me.push_devices}台
        </p>
        <Button type="button" variant="secondary" className="w-full" disabled={pending} onClick={() => void onEnablePush()}>
          この端末で通知を受け取る
        </Button>
        {me.push_devices > 0 ? (
          <Button type="button" variant="ghost" className="w-full" disabled={pending} onClick={() => void onTestPush()}>
            テストの通知を送る
          </Button>
        ) : null}
        {pushMessage ? <p className="text-sm text-slate-300">{pushMessage}</p> : null}
      </section>

      {me.demo ? (
        <section className="space-y-2">
          <h2 className="font-bold">申し込んだもの</h2>
          <Link href="/me/coaching/requests" className="block text-sm text-blue-300 underline">
            フォーム添削の依頼
          </Link>
          <Link href="/me/coaching/personal" className="block text-sm text-blue-300 underline">
            パーソナル指導の申し込み
          </Link>
        </section>
      ) : null}
    </div>
  );
}
