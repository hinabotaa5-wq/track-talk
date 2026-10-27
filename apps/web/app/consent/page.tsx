"use client";

import { useAuth } from "@/components/auth-provider";
import { OverseasConsentNotice } from "@/components/overseas-consent-notice";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { safeNextPath } from "@/lib/safe-next";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

// 海外の事業者に預けることへの同意を、今いる利用者から1回だけもらう画面。
// 同意しない人は使い続けられないので、退会の画面へ案内する。
function ConsentForm() {
  const { session, token, profile, loading, refreshProfile } = useAuth();
  const router = useRouter();
  const next = safeNextPath(useSearchParams().get("next"));
  const [checked, setChecked] = useState(false);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (loading) return;
    if (!session) router.replace("/login");
    else if (profile?.overseas_consent) router.replace(next);
  }, [loading, session, profile, next, router]);

  async function onConsent() {
    if (!checked || !acquire()) return;
    setError("");
    try {
      await apiFetch<void>("/v1/me/consents/overseas", { method: "POST", token });
      await refreshProfile();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "同意を記録できませんでした。時間をおいてもう一度お試しください。");
    } finally {
      release();
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">海外の事業者に預けることへの同意</h1>
      <p className="text-sm leading-7 text-slate-300">
        TrackTalk は、あなたの個人情報（アカウント、投稿、感覚メモ、ケガの記録など）を、米国の事業者のサーバーに預けて動いています。個人情報保護法にもとづき、続けて使う前に、次の説明を読んで同意してください。同意は1回だけで、次からは聞きません。
      </p>
      <OverseasConsentNotice />
      <label className="flex cursor-pointer items-start gap-2 text-sm">
        <input
          type="checkbox"
          className="mt-1 size-4 shrink-0 accent-blue-600"
          checked={checked}
          onChange={(e) => setChecked(e.target.checked)}
        />
        <span>
          上の説明と
          <Link href="/privacy" className="text-blue-300 underline">
            プライバシーポリシー
          </Link>
          第7条を読み、個人情報を米国などの海外の事業者に預けることに同意します。
        </span>
      </label>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="button" className="w-full" disabled={!checked || pending || !session} onClick={onConsent}>
        同意して続ける
      </Button>
      <p className="text-sm text-slate-300">
        同意しない場合は、TrackTalk を使い続けられません。
        <Link href="/me/delete" className="text-blue-300 underline">
          退会の手続き
        </Link>
        からアカウントとデータを削除できます。わからないことは
        <Link href="/contact" className="text-blue-300 underline">
          お問い合わせ
        </Link>
        ください。
      </p>
    </div>
  );
}

export default function ConsentPage() {
  return (
    <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
      <ConsentForm />
    </Suspense>
  );
}
