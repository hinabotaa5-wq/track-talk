"use client";

import { L } from "@/lib/i18n";
import { PageLoading } from "@/components/page-loading";
export function UploadOpening() {
  return <PageLoading screen label={L("投稿画面を開いています…")} />;
}
