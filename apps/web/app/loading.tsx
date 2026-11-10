"use client";

import { L } from "@/lib/i18n";
import { PageLoading } from "@/components/page-loading";
export default function Loading() {
  return <PageLoading screen label={L("画面を準備しています")} />;
}
