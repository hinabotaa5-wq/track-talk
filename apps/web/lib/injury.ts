export const injuryMoods = [
  { value: "vent", label: "吐き出し", hint: "つらい気持ちをそのまま書く" },
  { value: "rehab", label: "代替トレ", hint: "走れない中でやっていることを共有する" },
] as const;

export const injuryBodyParts = [
  { value: "foot", label: "足裏・足部" },
  { value: "ankle", label: "足首" },
  { value: "achilles", label: "アキレス腱" },
  { value: "calf", label: "ふくらはぎ" },
  { value: "shin", label: "すね" },
  { value: "knee", label: "膝" },
  { value: "quad", label: "太もも前" },
  { value: "hamstring", label: "ハムストリング" },
  { value: "groin", label: "股関節・鼠径部" },
  { value: "hip", label: "お尻・骨盤" },
  { value: "lower-back", label: "腰" },
  { value: "upper-body", label: "上半身" },
  { value: "other", label: "その他" },
] as const;

export const injuryStages = [
  { value: "resting", label: "安静中" },
  { value: "rehab", label: "リハビリ中" },
  { value: "jogging", label: "ジョグ再開" },
  { value: "practice", label: "練習に復帰" },
  { value: "race", label: "大会に復帰" },
] as const;

export type InjuryStage = (typeof injuryStages)[number]["value"];

export function isInjuryStage(value: string | null | undefined): value is InjuryStage {
  return injuryStages.some((item) => item.value === value);
}

export const injuryUpdateMaxLength = 2000;

export type InjuryMood = (typeof injuryMoods)[number]["value"];

export function isInjuryMood(value: string | null | undefined): value is InjuryMood {
  return injuryMoods.some((item) => item.value === value);
}

export function isInjuryBodyPart(value: string | null | undefined) {
  return injuryBodyParts.some((item) => item.value === value);
}

export const injuryNotice = "ここでの投稿は医療的な助言ではありません。痛みが続くときは医療機関に相談してください。";

export function injuryFeedPath(mood: string, part: string, page: number, stage = "") {
  const qs = new URLSearchParams({ kind: "injury", sort: "new", page: String(page), per_page: "20" });
  if (isInjuryMood(mood)) qs.set("mood", mood);
  if (isInjuryBodyPart(part)) qs.set("part", part);
  if (isInjuryStage(stage)) qs.set("stage", stage);
  return `/v1/posts?${qs}`;
}

export type InjuryStageStat = {
  stage: InjuryStage;
  stage_label: string;
  reached: number;
  median_days: number | null;
};

export type InjuryPartStat = {
  body_part: string;
  body_part_label: string;
  posts: number;
  stages: InjuryStageStat[];
};

export type InjuryStats = { min_sample: number; parts: InjuryPartStat[] };

// 中央値が出ている段階は日数、足りない段階はあと何人で目安が出るかを返す。
export function injuryStageStatText(stage: InjuryStageStat, minSample: number) {
  if (stage.median_days === null) {
    return `報告 ${stage.reached}人（あと${Math.max(minSample - stage.reached, 1)}人で目安を表示）`;
  }
  return stage.median_days === 0 ? `投稿から当日（${stage.reached}人）` : `投稿から約${stage.median_days}日（${stage.reached}人）`;
}
