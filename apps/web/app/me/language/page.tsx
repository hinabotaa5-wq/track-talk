"use client";

import { PageLoading } from "@/components/page-loading";
import { useLocale } from "@/components/locale-provider";
import { t } from "@/lib/i18n";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 表示言語の手動選択は廃止（運営者決定 2026-11-13）。
// 直接開いたときは 404 にせずマイページへ送る。
export default function LanguagePage() {
  const router = useRouter();
  const locale = useLocale();

  useEffect(() => {
    router.replace("/me");
  }, [router]);

  return <PageLoading screen label={t(locale, "common.loading")} />;
}
