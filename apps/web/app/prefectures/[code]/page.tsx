import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isPrefecturePageCode } from "@/lib/prefecture-page";
import { PREFECTURES, prefectureName } from "@/lib/prefectures";
import { PrefectureView } from "./prefecture-view";

// 中身は画面側で取るので、47都道府県ぶんをビルド時に作っておき、端のキャッシュから返す。
export const dynamicParams = false;

export function generateStaticParams() {
  return PREFECTURES.filter((item) => isPrefecturePageCode(item.code)).map((item) => ({ code: item.code }));
}

export async function generateMetadata({ params }: { params: Promise<{ code: string }> }): Promise<Metadata> {
  const { code } = await params;
  return { title: isPrefecturePageCode(code) ? `${prefectureName(code)}の大会と募集` : "都道府県" };
}

export default async function PrefecturePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  if (!isPrefecturePageCode(code)) notFound();
  return <PrefectureView code={code} />;
}
