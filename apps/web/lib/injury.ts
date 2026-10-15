// ケガ部のグループ。API とデータベースでは今も body_part と呼ぶが、1つのグループが1つのケガ(診断名)にあたる。
// 8つに当てはまらない人は「その他」に入り、ケガの名前を自由に書く。
export const injuryBodyParts = [
  { value: "hamstring-strain", label: "ハムストリングの肉離れ" },
  { value: "quad-strain", label: "太もも前の肉離れ" },
  { value: "sprain", label: "捻挫" },
  { value: "achilles-tendinitis", label: "アキレス腱炎" },
  { value: "jumpers-knee", label: "ジャンパー膝(膝蓋腱炎)" },
  { value: "shin-splints", label: "シンスプリント" },
  { value: "shoulder", label: "肩の痛み" },
  { value: "elbow", label: "肘の痛み" },
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
  // "YYYY-MM"。いつケガをしたかは、だいたいの月だけ聞く。
  injured_month: string;
  // 以下は選択式になる前に入った人にはない。
  diagnosis?: string;
  severity?: InjurySeverity;
  severity_label?: string;
  saw_doctor?: boolean;
  place?: InjuryPlace;
  place_label?: string;
  activity?: InjuryActivity;
  activity_label?: string;
  pre_changes: InjuryPreChange[];
  event?: InjuryEvent;
  event_label?: string;
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

// 入るときの選択肢。予防や復帰の目安に集計するので、自由記述ではなく選んでもらう。
export const injurySeverities = [
  { value: "mild", label: "軽い", detail: "練習しながら様子を見ている" },
  { value: "moderate", label: "中くらい", detail: "練習を数週間休む" },
  { value: "severe", label: "重い", detail: "1か月以上休む、または手術" },
] as const;
export type InjurySeverity = (typeof injurySeverities)[number]["value"];

export const injuryPlaces = [
  { value: "tartan", label: "タータン" },
  { value: "grass", label: "芝生" },
  { value: "dirt", label: "土" },
  { value: "asphalt", label: "アスファルト" },
  { value: "slope", label: "坂" },
  { value: "stairs", label: "階段" },
  { value: "weight-room", label: "ウエイト場" },
  { value: "gym", label: "体育館" },
] as const;
export type InjuryPlace = (typeof injuryPlaces)[number]["value"];

export const injuryActivities = [
  { value: "jog", label: "ジョグ" },
  { value: "start", label: "スタート・加速" },
  { value: "sprint", label: "中間から後半を全力疾走" },
  { value: "jump-hurdle", label: "ジャンプ・ハードル" },
  { value: "throw", label: "投てき" },
  { value: "weights", label: "ウエイト" },
  { value: "outside", label: "練習以外" },
] as const;
export type InjuryActivity = (typeof injuryActivities)[number]["value"];

export const injuryPreChanges = [
  { value: "more-volume", label: "練習量を増やした" },
  { value: "new-shoes", label: "シューズを替えた" },
  { value: "comeback", label: "休み明け・久しぶりの再開" },
  { value: "fatigue", label: "疲れがたまっていた" },
  { value: "played-through-pain", label: "痛みをがまんしていた" },
  { value: "none", label: "とくになし" },
] as const;
export type InjuryPreChange = (typeof injuryPreChanges)[number]["value"];

export const injuryEvents = [
  { value: "sprint", label: "短距離" },
  { value: "hurdles", label: "ハードル" },
  { value: "middle", label: "中距離" },
  { value: "long", label: "長距離" },
  { value: "jumps", label: "跳躍" },
  { value: "throws", label: "投てき" },
  { value: "combined", label: "混成" },
  { value: "race-walk", label: "競歩" },
] as const;
export type InjuryEvent = (typeof injuryEvents)[number]["value"];

export const injuryDiagnosisMaxLength = 40;

// 参加時に1問ずつ聞く質問。順番がそのまま画面の順。ケガの名前は「その他」のときだけ聞く。
export const injuryJoinSteps = [
  "diagnosis",
  "severity",
  "sawDoctor",
  "injuredMonth",
  "place",
  "activity",
  "preChanges",
  "event",
  "stage",
] as const;
export type InjuryJoinStep = (typeof injuryJoinSteps)[number];

export function injuryJoinStepsFor(part: string): InjuryJoinStep[] {
  return injuryJoinSteps.filter((step) => step !== "diagnosis" || part === "other");
}

export type InjuryJoinAnswers = {
  diagnosis: string;
  severity: InjurySeverity | "";
  sawDoctor: boolean | null;
  injuredMonth: string;
  place: InjuryPlace | "";
  activity: InjuryActivity | "";
  preChanges: InjuryPreChange[];
  event: InjuryEvent | "";
  stage: InjuryStage | "";
};

export const emptyInjuryJoinAnswers: InjuryJoinAnswers = {
  diagnosis: "",
  severity: "",
  sawDoctor: null,
  injuredMonth: "",
  place: "",
  activity: "",
  preChanges: [],
  event: "",
  stage: "",
};

// 端末の月を YYYY-MM で返す(「今月より先」を選べなくするため)。
export function localMonthString(now: Date) {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

// 今月から過去 count か月を、新しい順に返す。いつごろケガをしたかの選択肢。
export function recentMonths(now: Date, count = 24) {
  return Array.from({ length: count }, (_, i) => {
    const day = new Date(now.getFullYear(), now.getMonth() - i, 1);
    return { value: localMonthString(day), label: `${day.getFullYear()}年${day.getMonth() + 1}月` };
  });
}

// 複数選べる質問で、「なし」にあたる選択肢はほかと一緒に選べない。どちらかを選ぶと他方を外す。
function toggleExclusive<T extends string>(current: T[], value: T, exclusive: T): T[] {
  if (current.includes(value)) return current.filter((item) => item !== value);
  if (value === exclusive) return [exclusive];
  return [...current.filter((item) => item !== exclusive), value];
}

export function togglePreChange(current: InjuryPreChange[], value: InjuryPreChange): InjuryPreChange[] {
  return toggleExclusive(current, value, "none");
}

function hasValue(options: readonly { value: string }[], value: string) {
  return options.some((item) => item.value === value);
}

// その質問の答えに問題があれば、画面に出す文言を返す。問題なければ空文字。
export function injuryJoinStepError(step: InjuryJoinStep, answers: InjuryJoinAnswers, thisMonth: string) {
  switch (step) {
    case "diagnosis": {
      const length = [...answers.diagnosis.trim()].length;
      if (length === 0) return "ケガの名前を入力してください(わからなければ「わからない」)";
      if (length > injuryDiagnosisMaxLength) return `${injuryDiagnosisMaxLength}文字以内で入力してください`;
      return "";
    }
    case "severity":
      return hasValue(injurySeverities, answers.severity) ? "" : "ケガの重さを選んでください";
    case "sawDoctor":
      return answers.sawDoctor === null ? "病院でみてもらったかを選んでください" : "";
    case "injuredMonth":
      if (!/^\d{4}-\d{2}$/.test(answers.injuredMonth)) return "いつごろケガをしたかを選んでください";
      if (answers.injuredMonth > thisMonth) return "これから先の月は選べません";
      return "";
    case "place":
      return hasValue(injuryPlaces, answers.place) ? "" : "ケガをした場所を選んでください";
    case "activity":
      return hasValue(injuryActivities, answers.activity) ? "" : "そのとき何をしていたかを選んでください";
    case "preChanges":
      return answers.preChanges.length > 0 ? "" : "当てはまるものを選んでください(なければ「とくになし」)";
    case "event":
      return hasValue(injuryEvents, answers.event) ? "" : "種目を選んでください";
    case "stage":
      return isInjuryStage(answers.stage) ? "" : "いまの段階を選んでください";
  }
}

export function injuryJoinPayload(part: string, answers: InjuryJoinAnswers) {
  return {
    ...(part === "other" ? { diagnosis: answers.diagnosis.trim() } : {}),
    severity: answers.severity,
    saw_doctor: answers.sawDoctor,
    injured_month: answers.injuredMonth,
    place: answers.place,
    activity: answers.activity,
    pre_changes: answers.preChanges,
    event: answers.event,
    stage: answers.stage,
  };
}

// 段階を変えるときに聞く「意識的にやったこと」。復帰までに何をしたかを集計して、次の人のヒントにする。
export const injuryActions = [
  { value: "hospital-rehab", label: "病院のリハビリ" },
  { value: "osteopath", label: "整骨院" },
  { value: "stretch", label: "ストレッチ" },
  { value: "bath", label: "入浴" },
  { value: "strength", label: "筋トレ・補強" },
  { value: "icing", label: "アイシング" },
  { value: "taping", label: "テーピング" },
  { value: "rest", label: "できるだけ動かなかった" },
  { value: "nothing", label: "何もしなかった" },
] as const;
export type InjuryAction = (typeof injuryActions)[number]["value"];

export const injuryStageNoteMaxLength = 300;

export function toggleInjuryAction(current: InjuryAction[], value: InjuryAction): InjuryAction[] {
  return toggleExclusive(current, value, "nothing");
}

export type InjuryStageChangeAnswers = {
  stage: InjuryStage | "";
  actions: InjuryAction[];
  note: string;
  notePublic: boolean;
};

export const emptyInjuryStageChangeAnswers: InjuryStageChangeAnswers = { stage: "", actions: [], note: "", notePublic: false };

// 段階を変えるフォームの答えに問題があれば、最初の問題の文言を返す。問題なければ空文字。
export function injuryStageChangeError(answers: InjuryStageChangeAnswers, current: InjuryStage) {
  if (!isInjuryStage(answers.stage)) return "新しい段階を選んでください";
  if (answers.stage === current) return "いまと同じ段階です。変わった段階を選んでください";
  if (answers.actions.length === 0) return "意識的にやったことを1つ以上選んでください(なければ「何もしなかった」)";
  if ([...answers.note.trim()].length > injuryStageNoteMaxLength) return `一言は${injuryStageNoteMaxLength}文字以内で書いてください`;
  return "";
}

export function injuryStageChangePayload(answers: InjuryStageChangeAnswers) {
  const note = answers.note.trim();
  return { stage: answers.stage, actions: answers.actions, note, note_public: note !== "" && answers.notePublic };
}

// マイページに出す「ケガから約◯か月」。ケガをした日は月までしか聞いていないので、月の差で言う。
export function injuryElapsedText(injuredMonth: string, now: Date) {
  const match = /^(\d{4})-(\d{2})$/.exec(injuredMonth);
  if (!match) return "";
  const months = (now.getFullYear() - Number(match[1])) * 12 + now.getMonth() + 1 - Number(match[2]);
  if (months <= 0) return "今月ケガ";
  if (months < 12) return `ケガから約${months}か月`;
  const years = Math.floor(months / 12);
  const rest = months % 12;
  return rest === 0 ? `ケガから約${years}年` : `ケガから約${years}年${rest}か月`;
}

export function injuryGroupPath(part: string) {
  return `/injury/${part}`;
}

// 一覧の並び: injuryBodyParts の順で固定し、その他は最後。
export function sortInjuryGroups(groups: InjuryGroupSummary[]) {
  const order = new Map<string, number>(injuryBodyParts.map((item, index) => [item.value, index]));
  return [...groups].sort((a, b) => (order.get(a.body_part) ?? 0) - (order.get(b.body_part) ?? 0));
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

function hasCelebrated(part: string) {
  try {
    return localStorage.getItem(injuryCelebrationKey(part)) === "1";
  } catch {
    return false;
  }
}

// 祝うべきなら、祝ったことを覚えて true を返す。保存できなくても、祝うのが1回余計に出るだけ。
export function claimCelebration(part: string, stage: string) {
  if (!shouldCelebrate(stage, hasCelebrated(part))) return false;
  try {
    localStorage.setItem(injuryCelebrationKey(part), "1");
  } catch {
    // 保存できなくても続ける。
  }
  return true;
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
