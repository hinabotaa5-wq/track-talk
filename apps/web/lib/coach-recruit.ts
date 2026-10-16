import type { CoachEventGroup, CoachStatus, CoachTier } from "./types";

// コーチ募集の応募画面。
export const coachApplyPath = "/coach/apply";

export const coachTierLabels: Record<CoachTier, string> = {
  coach: "コーチ",
  senpai: "先輩(大学生・社会人)",
};

// 担当種目のグループ。並びは画面に出す順。
export const coachEventGroupLabels: Record<CoachEventGroup, string> = {
  sprint: "短短(100m・200m)",
  long_sprint: "短長(400m)",
  hurdles: "ハードル",
  distance: "中長距離",
  jumps: "跳躍",
  throws: "投擲",
};

export const coachEventGroups = Object.keys(coachEventGroupLabels) as CoachEventGroup[];

export const coachStatusLabels: Record<CoachStatus, string> = {
  pending: "審査中",
  approved: "承認済み",
  rejected: "見送り",
  suspended: "停止中",
};
