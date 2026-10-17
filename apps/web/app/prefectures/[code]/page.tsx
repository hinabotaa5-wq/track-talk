import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isPrefecturePageCode } from "@/lib/prefecture-page";
import { prefectureName } from "@/lib/prefectures";
import { PrefectureView } from "./prefecture-view";

export async function generateMetadata({ params }: { params: Promise<{ code: string }> }): Promise<Metadata> {
  const { code } = await params;
  return { title: isPrefecturePageCode(code) ? `${prefectureName(code)}の大会と募集` : "都道府県" };
}

export default async function PrefecturePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  if (!isPrefecturePageCode(code)) notFound();
  return <PrefectureView code={code} />;
}
