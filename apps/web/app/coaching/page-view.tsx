"use client";

import { CoachingGate } from "@/components/coaching-gate";
import { coachingCoachListEnabled, coachingEnabled } from "@/lib/features";
import { CoachesView } from "../coaches/page-view";

// コーチタブの着地（/coaching）はカードメニューではなく、/coaches と同じ一覧を出す。
// 二重実装にしないため、中身は CoachesView をそのまま共有する。
// ゲートは /coaches の layout と同じ条件をこのルート単体にだけ適用する
// （/coaching/* 配下の一覧フラグの扱いは変えない）。
export function CoachingView() {
  return (
    <CoachingGate announceVideoReviewStart open={coachingEnabled && coachingCoachListEnabled}>
      <CoachesView />
    </CoachingGate>
  );
}
