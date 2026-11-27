import { redirect } from "next/navigation";
import { featuredCoachIDs } from "@/lib/features";

// 特集モード（本体 #985）では、特集コーチ（複数可、本体 #1063）以外の詳細（直URL・相談の入口を含む）を特集ページへ戻す。
// API も特集コーチ以外を 404 にするので、見た目だけの出し分けではない。
export default async function CoachDetailLayout({ children, params }: { children: React.ReactNode; params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (featuredCoachIDs.length && !featuredCoachIDs.includes(decodeURIComponent(id))) redirect("/coaching");
  return <>{children}</>;
}
