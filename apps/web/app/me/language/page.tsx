"use client";

import { PageLoading } from "@/components/page-loading";
import { useAuth } from "@/components/auth-provider";
import { LanguageSwitch } from "@/components/language-switch";
import { useLocale } from "@/components/locale-provider";
import { t } from "@/lib/i18n";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// マイページの設定一覧から開く「表示言語 / Language」（本体 #536）。他の設定と同じく1つ奥の画面に置く。
export default function LanguagePage() {
  const { session, loading } = useAuth();
  const router = useRouter();
  const locale = useLocale();

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/language");
  }, [loading, session, router]);

  if (loading || !session) {
    return <PageLoading screen label={t(locale, "common.loading")} />;
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 id="language-page-title" className="text-2xl font-bold">{t(locale, "profile.language")}</h1>
      <LanguageSwitch labelledBy="language-page-title" />
    </div>
  );
}
