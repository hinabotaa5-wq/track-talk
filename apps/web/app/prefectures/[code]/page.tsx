import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { apiFetch } from "@/lib/api";
import { isPrefecturePageCode, type PrefecturePage } from "@/lib/prefecture-page";
import { PREFECTURES, prefectureName } from "@/lib/prefectures";
import { PrefectureView } from "./prefecture-view";

// 47都道府県だけを受け付ける。中身は開くたびにサーバーで描き、検索エンジンが読めるようにする。
export const dynamicParams = false;

export function generateStaticParams() {
  return PREFECTURES.filter((item) => isPrefecturePageCode(item.code)).map((item) => ({ code: item.code }));
}

export async function generateMetadata({ params }: { params: Promise<{ code: string }> }): Promise<Metadata> {
  const { code } = await params;
  if (!isPrefecturePageCode(code)) return { title: "都道府県" };
  const name = prefectureName(code);
  return {
    title: `${name}の陸上競技の大会日程`,
    description: `${name}でこれから開かれる陸上競技の大会(記録会・選手権など)の日程をまとめています。`,
    alternates: { canonical: `/prefectures/${code}` },
  };
}

export default async function PrefectureRoute({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  if (!isPrefecturePageCode(code)) notFound();
  const page = await apiFetch<PrefecturePage>(`/v1/prefectures/${code}`, { signal: AbortSignal.timeout(3000) }).catch(
    () => null,
  );
  return <PrefectureView code={code} initialPage={page} />;
}
