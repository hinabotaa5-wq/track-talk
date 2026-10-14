import { Suspense } from "react";
import { InjuryView } from "./injury-view";

export default function InjuryPage() {
  return (
    <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
      <InjuryView />
    </Suspense>
  );
}
