import type { Metadata } from "next";
import { InjuryHome } from "../injury-home";

export const metadata: Metadata = {
  title: "ケガのデータ｜ケガの状況と、復帰までにやったこと",
  description:
    "ケガをしたときの状況と、復帰までにやったことを、共有のグループの回答から匿名で集計。まだケガをしていない人の予防に役立てます。",
  alternates: { canonical: "/injury/data" },
};

export default function InjuryDataPage() {
  return (
    <div className="min-h-svh md:min-h-0">
      <InjuryHome initialView="data" />
    </div>
  );
}
