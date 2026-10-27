"use client";

import { useAuth } from "@/components/auth-provider";
import { ServiceConsentText } from "@/components/service-consent-text";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { safeNextPath } from "@/lib/safe-next";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

// 登録画面を通らずに入った人も、規約などと海外への提供を1回で同意する画面。
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
      <h1 className="text-2xl font-bold">利用にあたっての同意</h1>
      <p className="text-sm leading-7 text-slate-300">
        TrackTalk を利用する前に、以下の内容を確認してください。海外への個人情報提供の詳細は、プライバシーポリシー第7条に記載しています。
      </p>
      <label className="flex cursor-pointer items-start gap-2 text-sm">
        <input
          type="checkbox"
          className="mt-1 size-4 shrink-0 accent-blue-600"
          checked={checked}
          onChange={(e) => setChecked(e.target.checked)}
        />
        <ServiceConsentText />
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
