import type { CoachEvent, CoachStatus, CoachTier } from "./types";

// コーチ募集の応募画面。
export const coachApplyPath = "/coach/apply";

export const coachTierLabels: Record<CoachTier, string> = {
  coach: "コーチ",
  senpai: "社会人・大学生",
};

// 担当種目。並びは画面に出す順。
export const coachEventLabels: Record<CoachEvent, string> = {
  "100m": "100m",
  "200m": "200m",
  "400m": "400m",
  "800m": "800m",
  "1500m": "1500m",
  "5000m": "5000m",
  "10000m": "10000m",
  "110mh": "110mハードル",
  "100mh": "100mハードル",
  "400mh": "400mハードル",
  "3000msc": "3000m障害物",
  "high-jump": "走高跳",
  "pole-vault": "棒高跳",
  "long-jump": "走幅跳",
  "triple-jump": "三段跳",
  "shot-put": "砲丸投",
  "discus": "円盤投",
  "hammer": "ハンマー投",
  "javelin": "やり投",
  "race-walk": "競歩",
  "relay": "リレー",
  "heptathlon": "七種競技",
  "decathlon": "十種競技",
};

export const coachEvents = Object.keys(coachEventLabels) as CoachEvent[];

export const coachStatusLabels: Record<CoachStatus, string> = {
  pending: "審査中",
  approved: "承認済み",
  rejected: "見送り",
  suspended: "停止中",
};
