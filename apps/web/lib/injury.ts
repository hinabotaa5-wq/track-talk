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

export function isInjuryBodyPart(value: string | null | undefined) {
  return injuryBodyParts.some((item) => item.value === value);
}

export const injuryNotice = "ここでの投稿は医療的な助言ではありません。痛みが続くときは医療機関に相談してください。";

export function injuryFeedPath(part: string, page: number, stage = "") {
  const qs = new URLSearchParams({ kind: "injury", sort: "new", page: String(page), per_page: "20" });
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

// 指定した部位のうち、日数の目安が出ている段階だけを返す。部位のデータがなければ空。
export function injuryTypicalStages(stats: InjuryStats | null, bodyPart: string) {
  const part = stats?.parts.find((item) => item.body_part === bodyPart);
  return part ? part.stages.filter((stage) => stage.median_days !== null) : [];
}

export type InjuryHistoryItem = {
  id: string;
  title: string;
  body_part: string;
  body_part_label: string;
  created_at: string;
  stages: { stage: InjuryStage; stage_label: string; days: number }[];
};

// 履歴の1行に出す、段階ごとの日数。当日なら「当日」。
export function injuryHistoryStageText(stage: { stage_label: string; days: number }) {
  return stage.days === 0 ? `${stage.stage_label}（投稿の当日）` : `${stage.stage_label}（${stage.days}日後）`;
}

// 同じ部位のケガが複数あるとき、その部位名と件数を返す（再発の振り返り用）。
export function injuryRepeatedParts(items: { body_part: string; body_part_label: string }[]) {
  const counts = new Map<string, { label: string; count: number }>();
  for (const item of items) {
    const current = counts.get(item.body_part);
    counts.set(item.body_part, { label: item.body_part_label, count: (current?.count ?? 0) + 1 });
  }
  return [...counts.values()].filter((item) => item.count >= 2);
}

export function injuryBodyPartLabel(code: string) {
  return injuryBodyParts.find((item) => item.value === code)?.label ?? code;
}

export type InjuryGroupMember = {
  stage: InjuryStage;
  stage_label: string;
  injured_on: string;
  place: string;
  situation: string;
};

export type InjuryGroupSummary = {
  body_part: string;
  body_part_label: string;
  members: number;
  me?: InjuryGroupMember;
};

export type InjuryGroupStage = {
  stage: InjuryStage;
  stage_label: string;
  members: number;
  faces: string[];
};

export type InjuryGroup = {
  body_part: string;
  body_part_label: string;
  members: number;
  stages: InjuryGroupStage[];
  me: InjuryGroupMember;
};

// 参加時に1問ずつ聞く質問。順番がそのまま画面の順。
export const injuryJoinSteps = ["injuredOn", "place", "situation", "stage"] as const;
export type InjuryJoinStep = (typeof injuryJoinSteps)[number];

export const injuryJoinPlaces = ["練習中", "大会・試合中", "日常生活", "ジム・筋トレ", "その他"] as const;

export const injuryJoinPlaceMaxLength = 100;
export const injuryJoinSituationMaxLength = 500;

export type InjuryJoinAnswers = {
  injuredOn: string;
  place: string;
  situation: string;
  stage: InjuryStage | "";
};

export const emptyInjuryJoinAnswers: InjuryJoinAnswers = { injuredOn: "", place: "", situation: "", stage: "" };

// 端末の日付を YYYY-MM-DD で返す(「今日より先」を選べなくするため)。
export function localDateString(now: Date) {
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

// その質問の答えに問題があれば、画面に出す文言を返す。問題なければ空文字。
export function injuryJoinStepError(step: InjuryJoinStep, answers: InjuryJoinAnswers, today: string) {
  switch (step) {
    case "injuredOn":
      if (!/^\d{4}-\d{2}-\d{2}$/.test(answers.injuredOn)) return "いつ痛めたかを選んでください";
      if (answers.injuredOn > today) return "未来の日付は選べません";
      return "";
    case "place":
      if (answers.place.trim() === "") return "どこで痛めたかを入力してください";
      if ([...answers.place.trim()].length > injuryJoinPlaceMaxLength) return `${injuryJoinPlaceMaxLength}文字以内で入力してください`;
      return "";
    case "situation":
      if (answers.situation.trim() === "") return "どんな状況だったかを入力してください";
      if ([...answers.situation.trim()].length > injuryJoinSituationMaxLength) return `${injuryJoinSituationMaxLength}文字以内で入力してください`;
      return "";
    case "stage":
      return isInjuryStage(answers.stage) ? "" : "いまの状況を選んでください";
  }
}

export function injuryJoinPayload(answers: InjuryJoinAnswers) {
  return {
    injured_on: answers.injuredOn,
    place: answers.place.trim(),
    situation: answers.situation.trim(),
    stage: answers.stage,
  };
}

export function injuryGroupPath(part: string) {
  return `/injury/${part}`;
}

// 一覧の並び: 参加中のグループを先に、そのあとは人の多い順(同数なら部位の並び順)。
export function sortInjuryGroups(groups: InjuryGroupSummary[]) {
  const order = new Map<string, number>(injuryBodyParts.map((item, index) => [item.value, index]));
  return [...groups].sort((a, b) => {
    if (!!a.me !== !!b.me) return a.me ? -1 : 1;
    if (a.members !== b.members) return b.members - a.members;
    return (order.get(a.body_part) ?? 0) - (order.get(b.body_part) ?? 0);
  });
}

// ロードマップ上の自分の段階だけ投稿できる。ほかの段階は読むだけ。
export function canPostInStage(selected: string, mine: string) {
  return selected === mine;
}

export function injuryCelebrationKey(part: string) {
  return `injury-celebrated:${part}`;
}

// 初めてゴール(大会に復帰)に着いたときだけ祝う。戻ってからの2回目は出さない。
export function shouldCelebrate(stage: string, alreadyCelebrated: boolean) {
  return stage === "race" && !alreadyCelebrated;
}

// ニックネームから、毎回同じアバターの色を決める。
export function injuryFaceHue(name: string) {
  let hash = 0;
  for (const char of name) hash = (hash * 31 + (char.codePointAt(0) ?? 0)) % 360;
  return hash;
}

export function injuryFaceInitial(name: string) {
  return [...name.trim()][0] ?? "?";
}

// ロードマップの人数の表示。顔に出し切れない分は「+N」にする。
export function injuryFaceOverflow(members: number, faces: number) {
  return Math.max(members - faces, 0);
}
