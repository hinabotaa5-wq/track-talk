// ケガ部の「データ」タブ。状況と「どうすればよかったか」を選択式だけで集めた、匿名の集計。
export type InjuryCaseChoice = { value: string; label: string };

export type InjuryCaseOptions = {
  timings: InjuryCaseChoice[];
  situations: InjuryCaseChoice[];
  preventions: InjuryCaseChoice[];
  max_choices: number;
};

export type InjuryChoiceStat = { code: string; label: string; count: number; percent: number };

export type InjuryCaseGroup = {
  body_part: string;
  body_part_label: string;
  cases: number;
  // false の間は、1人の回答が見えてしまうので、選択肢ごとの数は空。
  ready: boolean;
  timings: InjuryChoiceStat[];
  situations: InjuryChoiceStat[];
  preventions: InjuryChoiceStat[];
};

export type InjuryCaseStats = { min_sample: number; overall: InjuryCaseGroup; parts: InjuryCaseGroup[] };

export type InjuryMyCase = {
  id: string;
  body_part: string;
  body_part_label: string;
  timing_label: string;
  situations: string[];
  preventions: string[];
  created_at: string;
};

// 選択肢を、押すたびに入れる・外す。上限に届いていたら、新しい選択は入れない。
export function toggleChoice(selected: string[], value: string, max: number) {
  if (selected.includes(value)) return selected.filter((item) => item !== value);
  return selected.length >= max ? selected : [...selected, value];
}

export function injuryCaseNeedText(group: InjuryCaseGroup, minSample: number) {
  return `回答 ${group.cases}人（あと${Math.max(minSample - group.cases, 1)}人で集計を表示）`;
}
