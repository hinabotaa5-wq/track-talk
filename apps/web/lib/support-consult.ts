import type { CoachEvent } from "./types";

// 大会までプランの無料相談。API（support_consult.go）の値と合わせる。
// 相談はチャットにせず、運営が用意した質問への選択と、必要最小限の記入だけで行う。

export type ReceptionStatus = "open" | "paused";

// コーチ一覧・詳細に出す入口。受付中か受付停止中だけ（コーチが手で切り替える）。契約件数・残り枠・満枠は出さない。
export type PublicSupportConsult = {
  events: CoachEvent[];
  description: string;
  price_min_yen: number | null;
  price_max_yen: number | null;
  fee_percent?: number;
  reception: { status: ReceptionStatus; resume_on: string | null };
};

export type SupportConsultStatus = "requested" | "clarifying" | "proposed" | "sent_to_guardian" | "accepted" | "declined" | "expired" | "withdrawn";

export const supportConsultStatusLabels: Record<SupportConsultStatus, string> = {
  requested: "コーチの提案を待っています",
  clarifying: "コーチの聞き直しに答えてください",
  proposed: "コーチの提案が届いています",
  sent_to_guardian: "保護者に提案を送りました。保護者の同意を待っています",
  accepted: "提案を受けて申し込みました",
  declined: "コーチが辞退しました",
  expired: "期限が過ぎました",
  withdrawn: "取り下げました",
};

export type SupportConsultAnswers = {
  goal_decided: boolean | null;
  competition_name: string;
  competition_date: string;
  timing_hint: string;
  event: string;
  goals: string[];
  target_record: string;
  personal_best: string;
  pb_when: string;
  form: string;
  club: boolean | null;
  has_coach: boolean | null;
  menu_by: string;
  practice_days: number;
  session_minutes: number;
  track_access: string;
  weights_access: string;
  can_film: string;
  injury_now: boolean | null;
  injury_parts: string[];
  injury_note: string;
  injury_past_year: string;
  rest_period: boolean | null;
  focus: string[];
  main_worry: string;
  has_video: boolean | null;
  want_menu: boolean | null;
  want_video_reply: string;
  want_peak: boolean | null;
  start_timing: string;
  start_date: string;
  budget: string;
  question_to_coach: string;
};

export function emptyAnswers(): SupportConsultAnswers {
  return {
    goal_decided: null, competition_name: "", competition_date: "", timing_hint: "", event: "", goals: [], target_record: "",
    personal_best: "", pb_when: "", form: "", club: null, has_coach: null, menu_by: "", practice_days: 0, session_minutes: 0,
    track_access: "", weights_access: "", can_film: "", injury_now: null, injury_parts: [], injury_note: "", injury_past_year: "",
    rest_period: null, focus: [], main_worry: "", has_video: null, want_menu: null, want_video_reply: "", want_peak: null,
    start_timing: "", start_date: "", budget: "", question_to_coach: "",
  };
}

export type Choice = { value: string; label: string };
const c = (pairs: [string, string][]): Choice[] => pairs.map(([value, label]) => ({ value, label }));

export const timingHintChoices = c([["within_1m", "1か月以内"], ["1_3m", "1〜3か月先"], ["3_6m", "3〜6か月先"], ["over_6m", "半年より先"]]);
export const goalChoices = c([["pb", "自己ベストを出す"], ["final", "決勝・上位大会に進む"], ["qualify", "標準記録・参加資格を取る"], ["podium", "入賞・優勝をねらう"], ["experience", "経験を積む・楽しむ"]]);
export const pbWhenChoices = c([["within_3m", "3か月以内"], ["within_6m", "半年以内"], ["within_1y", "1年以内"], ["older", "それより前"]]);
export const formChoices = c([["good", "良い"], ["normal", "ふつう"], ["poor", "あまり良くない"]]);
export const menuByChoices = c([["self", "自分で決めている"], ["coach", "顧問・コーチが決めている"], ["other_person", "その他の人（先輩・家族など）"]]);
export const practiceDaysChoices = [1, 2, 3, 4, 5, 6, 7];
export const sessionMinutesChoices: Choice[] = c([["30", "30分"], ["60", "60分"], ["90", "90分"], ["120", "120分"], ["150", "150分以上"]]);
export const accessChoices = c([["yes", "使える"], ["sometimes", "ときどき使える"], ["no", "使えない"]]);
export const filmChoices = c([["yes", "撮れる"], ["sometimes", "ときどき撮れる"], ["no", "撮れない"]]);
export const injuryPartChoices = c([["foot_ankle", "足・足首"], ["lower_leg", "すね・ふくらはぎ"], ["knee", "ひざ"], ["thigh", "もも"], ["hip_groin", "股関節・そけい部"], ["back", "腰・背中"], ["upper_body", "肩・腕"], ["other", "その他"]]);
export const injuryPastChoices = c([["none", "なし"], ["minor", "軽い怪我があった"], ["major", "大きな怪我があった"]]);
export const focusChoices = c([["start", "スタート"], ["acceleration", "加速"], ["midrace", "中盤"], ["finish", "後半・フィニッシュ"], ["ground_contact", "接地"], ["arm_swing", "腕振り"], ["posture", "姿勢"], ["pitch_stride", "ピッチとストライド"], ["blocks", "スタートブロック"], ["other", "その他"]]);
export const videoReplyChoices = c([["none", "いらない"], ["sometimes", "ときどき"], ["often", "こまめに"]]);
export const startTimingChoices = c([["asap", "できるだけ早く"], ["date", "日付を指定する"]]);
export const budgetChoices = c([["under_5000", "5,000円未満"], ["5000_10000", "5,000〜10,000円"], ["10000_20000", "10,000〜20,000円"], ["over_20000", "20,000円より上"]]);

export function choiceLabel(choices: Choice[], value: string | number | null | undefined): string {
  return choices.find((x) => x.value === String(value))?.label ?? "";
}
export function choiceLabels(choices: Choice[], values: string[]): string {
  return values.map((v) => choiceLabel(choices, v)).filter(Boolean).join("・");
}

const yn = (v: boolean | null | undefined) => (v === true ? "はい" : v === false ? "いいえ" : "");

// answerRows は、コーチが読む回答を1項目1行にする。空の項目は出さない。
export function answerRows(a: SupportConsultAnswers, eventLabel: string): { section: string; label: string; value: string }[] {
  const rows: { section: string; label: string; value: string }[] = [];
  const add = (section: string, label: string, value: string | undefined) => {
    if (value) rows.push({ section, label, value });
  };
  const A = "目標の大会";
  add(A, "大会", a.goal_decided ? `${a.competition_name}（${a.competition_date}）` : `未定（${choiceLabel(timingHintChoices, a.timing_hint)}）`);
  add(A, "種目", eventLabel);
  add(A, "大会での目標", choiceLabels(goalChoices, a.goals));
  add(A, "目標記録", a.target_record);
  const B = "いまの実力";
  add(B, "自己ベスト", a.personal_best);
  add(B, "いつの記録か", choiceLabel(pbWhenChoices, a.pb_when));
  add(B, "最近の調子", choiceLabel(formChoices, a.form));
  const C = "練習環境";
  add(C, "部活所属", yn(a.club));
  add(C, "指導者", yn(a.has_coach));
  add(C, "メニューを決める人", choiceLabel(menuByChoices, a.menu_by));
  add(C, "週の練習日数", a.practice_days ? `${a.practice_days}日` : "");
  add(C, "1回の練習時間", choiceLabel(sessionMinutesChoices, a.session_minutes));
  add(C, "競技場", choiceLabel(accessChoices, a.track_access));
  add(C, "ウエイト器具", choiceLabel(accessChoices, a.weights_access));
  add(C, "動画の撮影", choiceLabel(filmChoices, a.can_film));
  const D = "体と制約";
  add(D, "今の痛み・怪我", a.injury_now ? `あり（${choiceLabels(injuryPartChoices, a.injury_parts)}）${a.injury_note}` : yn(a.injury_now));
  add(D, "過去1年の大きな怪我", choiceLabel(injuryPastChoices, a.injury_past_year));
  add(D, "休む・減らす時期", yn(a.rest_period));
  const E = "直したいこと";
  add(E, "場面・要素", choiceLabels(focusChoices, a.focus));
  add(E, "一番の悩み", a.main_worry);
  add(E, "フォーム動画", a.has_video ? "あり" : "なし");
  const F = "希望するサポート";
  add(F, "メニュー作成", yn(a.want_menu));
  add(F, "動画への返答", choiceLabel(videoReplyChoices, a.want_video_reply));
  add(F, "大会直前の調整", yn(a.want_peak));
  add(F, "開始時期", a.start_timing === "date" ? a.start_date : "できるだけ早く");
  add(F, "予算の目安", choiceLabel(budgetChoices, a.budget));
  add(F, "コーチに聞きたいこと", a.question_to_coach);
  return rows;
}

export type SupportConsultProposal = {
  competition_name: string;
  support_start_date: string;
  competition_date: string;
  duration_days: number;
  call_count: number;
  call_minutes: number;
  price_yen: number;
  fee_percent: number;
  coach_receives_yen: number;
  content_note: string;
  answer_to_question: string;
  valid_until: string;
  // 有効期限の最後の日（日本時間・この日の終わりまで）。
  valid_until_date: string;
  expired?: boolean;
};

export type SupportConsult = {
  id: string;
  status: SupportConsultStatus;
  minor: boolean;
  event: string;
  answers: SupportConsultAnswers;
  answer_by: string | null;
  contract_id: string | null;
  created_at: string;
  updated_at: string;
  role?: "athlete" | "coach";
  coach_id?: string;
  coach_nickname?: string;
  athlete_nickname?: string;
  questions?: { id: string; question: string; answer: string | null; asked_at: string; answered_at: string | null }[];
  proposal?: SupportConsultProposal | null;
};

export type SupportConsultSettings = {
  accepting: boolean;
  events: CoachEvent[];
  description: string;
  price_min_yen: number | null;
  price_max_yen: number | null;
};

// 契約と相談は提案ではまだ成立しない、という前置き（フォームの冒頭と提案画面に出す）。
export const consultNotContractNote = "ここでの相談と提案は契約ではありません。契約は、提案を受けて申し込み（18歳未満は、保護者が提案の内容に同意して申し込みます）、お支払いが済んだ時点で成立します。コーチは提案を出した時に同意しているので、あらためて引き受ける操作はありません。相談だけなら費用はかからず、カード情報も入力しません。";
// 大会までプランの「何が返ってくるか」の標準形。提案の内容欄にいつも明記する（API の supportStandardContent と同じ文）。
export const supportStandardContent = "動画への返答は、期間中は回数の制限なく受けられます。メニューはコーチが先に作り、必要に応じて見直します。";
export const consultMinorNote = "18歳未満の方は、あなたが申し込むのではなく、保護者に提案を送ります。契約には保護者の同意とお支払いが必要です。";

export function priceHintLabel(min: number | null, max: number | null): string {
  if (min == null || max == null) return "";
  const f = (n: number) => `${n.toLocaleString("ja-JP")}円`;
  return `${f(min)}〜${f(max)}`;
}

// 大会までプランを途中で終えたときの日割りの算定例。API の supportUnprovidedAsOf と同じ計算
// （契約価格 ÷ 期間日数 × 未経過日数、円未満切り捨て。終える日は経過に数える）。
export function supportProrationExample(priceYen: number, days: number) {
  const total = Math.max(1, Math.floor(days));
  const elapsed = Math.min(total, Math.max(1, Math.floor(total / 3)));
  const remaining = total - elapsed;
  const refund = Math.floor((priceYen * remaining) / total);
  return { total, elapsed, remaining, refund };
}

// 契約が終わったあとにチャットを見られる日数（閲覧のみ・送信はできない）。
export const supportChatViewDays = 90;

// 提案画面と最終確認画面に出す、途中で終わる場合のお金とチャットの扱い。
export function supportTermsLines(priceYen: number, days: number): { label: string; text: string }[] {
  const ex = supportProrationExample(priceYen, days);
  const yen = (n: number) => `${n.toLocaleString("ja-JP")}円`;
  return [
    {
      label: "途中で終わる場合（日割り）",
      text: `解約料・違約金はかかりません。まだ過ぎていない日数分を日割りで全額返金します（契約価格 ÷ 期間の日数 × 残りの日数、1円未満は切り捨て。期間は実際に始まった日から目標大会の日までで、終える日は過ぎた日に数えます）。例: ${yen(priceYen)}・${ex.total}日間の契約を${ex.elapsed}日目に終えると、残り${ex.remaining}日分の${yen(ex.refund)}を返金します。始まる前に終えた場合は全額を返金します。`,
    },
    {
      label: "コーチの都合で続けられない場合",
      text: "コーチの責任で続けられなくなった場合は、過ぎた日数分も含めて全額を返金します。病気・災害などやむを得ない事情の場合は、上の日割りの返金になります。",
    },
    {
      label: "返金の時期と方法",
      text: "終了の手続きをした時点で、お支払いに使ったカードへ返金します。明細に反映されるまでの日数はカード会社によって異なります（通常は数日から2週間ほど）。",
    },
    {
      label: "大会が中止・延期になった場合",
      text: "主催者が中止・延期を正式に発表した日で契約を終えます。別の大会への振り替えや期間の延長はしません。発表の日までを過ぎた日として、残りの日数分を日割りで返金します。次の大会に向けては、あらためて無料相談ができます。",
    },
    {
      label: "契約後のチャット",
      text: `コーチからの返答は24時間以内を目安としています。契約が終わった後も${supportChatViewDays}日間はチャットを見ることができます（送信はできません）。終了時にやりとり（文章とメニュー）をダウンロードできます。${supportChatViewDays}日を過ぎると見られなくなります。`,
    },
  ];
}
