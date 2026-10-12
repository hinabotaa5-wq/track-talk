import { Suspense } from "react";
import { CompetitionsView } from "./competitions-view";

export default function CompetitionsPage() {
  return (
    <Suspense fallback={<p className="text-sm text-zinc-600">読み込み中...</p>}>
      <CompetitionsView />
    </Suspense>
  );
}
