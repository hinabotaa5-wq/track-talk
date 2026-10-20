import type { Metadata } from "next";
import { DataView } from "./data-view";

export const metadata: Metadata = {
  title: "ケガのデータ｜ケガの状況と、復帰までにやったこと",
  description:
    "ケガをしたときの状況と、復帰までにやったことを、ケガ共有のグループの回答から匿名で集計。まだケガをしていない人の予防に役立てます。",
  alternates: { canonical: "/injury/data" },
};

export default function InjuryDataPage() {
  return (
    <div className="min-h-svh md:min-h-0">
      <DataView />
    </div>
  );
}
