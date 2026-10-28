import { formatDateTime, formatYen } from "./format";
import type {
  CoachingContract,
  CoachingContractStatus,
  CoachingPlan,
  CoachingPlanKind,
  CoachingServiceType,
  PlanIncludes,
  PlanMenuUpdates,
  PlanReplyFormat,
} from "./types";

// パーソナル指導のプランの表示と入力のきまり。API（coaching_plan.go）の値と合わせる。

export const coachingServiceLabels: Record<CoachingServiceType, string> = {
  video_review: "動画添削",
  competition_support: "大会までのサポート",
  in_person: "対面指導",
};

export function serviceReady(service: CoachingServiceType | null | undefined): boolean {
  return service !== "video_review";
}

function serviceLabel(service: CoachingServiceType | null | undefined): string {
  return service ? coachingServiceLabels[service] : "従来のサポートプラン";
}

export const planKindLabels: Record<CoachingPlanKind, string> = {
  period: "期間プラン",
  single: "単発",
  trial: "お試し",
};

// 種類ごとに選べる日数。単発とお試しは7日だけ。
export const planDurations: Record<CoachingPlanKind, number[]> = {
  period: [14, 30],
  single: [7],
  trial: [7],
};

export const minPlanPriceYen = 2000;
// これを超える値段は、保存する前に打ち間違いでないか確かめる。
export const highPlanPriceYen = 30000;
// お試しの値段の上限。
export const maxTrialPriceYen = 3000;

export function planDurationLabel(kind: CoachingPlanKind, days: number): string {
  if (kind === "single" || kind === "trial") return `${days}日以内`;
  if (days === 14) return "2週間";
  if (days === 30) return "1か月";
  return `${days}日`;
}

export function planSummary(plan: Pick<CoachingPlan, "kind" | "duration_days" | "price_yen">): string {
  return `${planKindLabels[plan.kind]} · ${planDurationLabel(plan.kind, plan.duration_days)} · ${formatYen(plan.price_yen)}`;
}

export const planMenuUpdatesLabels: Record<PlanMenuUpdates, string> = {
  none: "なし",
  once: "最初に1回作る",
  weekly: "毎週見直す",
};

export const planReplyFormatLabels: Record<PlanReplyFormat, string> = {
  text: "文章",
  video: "解説の動画",
};

// フォームで選べる動画への返答の回数。0 は回数を決めない。
export const planVideoReviewChoices = [1, 2, 4, 8, 0];

export function videoReviewsLabel(n: number): string {
  return n === 0 ? "回数を決めない（期間中いつでも）" : `${n}回まで`;
}

// planIncludes は、「何が返ってくるか」を1項目1行にする。書いていない項目は出さない。
export function planIncludes(p: PlanIncludes): { label: string; value: string }[] {
  const out: { label: string; value: string }[] = [];
  if (p.service_type !== undefined) out.push({ label: "サービス", value: serviceLabel(p.service_type) });
  if (p.additional_consultation_price_yen != null) out.push({ label: "追加相談の料金", value: `${formatYen(p.additional_consultation_price_yen)}／回` });
  if (p.competition_name) out.push({ label: "大会", value: p.competition_name });
  if (p.support_start_date && p.competition_date) out.push({ label: "支援日程", value: `${p.support_start_date} 〜 ${p.competition_date}（大会当日まで）` });
  if (p.call_count) out.push({ label: "ビデオ通話", value: `${p.call_count}回・1回${p.call_minutes ?? 30}分` });
  if (p.service_type === "competition_support") out.push({ label: "未提供分の返金", value: "動画回答1組・実施確認済み通話1回を各1単位として按分。終了48時間後に精算" });
  if (p.service_type === "in_person") {
    if (p.venue_area) out.push({ label: "地域", value: p.venue_area });
    if (p.venue_name) out.push({ label: "会場", value: p.venue_name });
    if (p.session_count) out.push({ label: "対面指導", value: `${p.session_count}回${p.session_minutes ? `・1回${p.session_minutes}分` : ""}` });
    if (p.travel_fee_yen != null) out.push({ label: "交通費", value: `${formatYen(p.travel_fee_yen)}（基本料金に加算）` });
    if (p.max_participants) out.push({ label: "募集人数", value: `${p.max_participants}人まで` });
  }
  if (p.video_reviews !== null) out.push({ label: "動画への返答", value: videoReviewsLabel(p.video_reviews) });
  if (p.menu_updates !== null) out.push({ label: "練習メニュー", value: planMenuUpdatesLabels[p.menu_updates] });
  if (p.reply_formats.length > 0) out.push({ label: "返し方", value: p.reply_formats.map((f) => planReplyFormatLabels[f]).join("・") });
  if (p.start_on_first_video) out.push({ label: "期間", value: "最初の動画を送った日から数える" });
  return out;
}

// 最初の動画から数えるプランで、動画を待つ日数。API（coaching_contract.sql）と合わせる。
export const firstVideoWaitDays = 7;

// contractPeriodLabel は、期間中・終わった申し込みの期間の表示。
// 最初の動画から数えるプランで、まだ動画を待っているあいだは、いつまでに送ればよいかを出す。
export function contractPeriodLabel(
  c: Pick<CoachingContract, "starts_at" | "ends_at" | "duration_days" | "start_on_first_video" | "clock_started_at" | "status">,
  now: Date,
): string {
  if (!c.starts_at || !c.ends_at) return "";
  if (c.start_on_first_video && !c.clock_started_at) {
    // 待つ日数ぶん先に延ばしてある終了日から期間を引いた時が、数え始めの期限。
    const waitUntil = new Date(new Date(c.ends_at).getTime() - c.duration_days * 24 * 60 * 60 * 1000);
    if (c.status === "active" && now < waitUntil) {
      return `最初の動画を送った日から${c.duration_days}日（${formatDateTime(waitUntil.toISOString())}までに送らなければ、その時から数えます）`;
    }
    return `${formatDateTime(waitUntil.toISOString())} 〜 ${formatDateTime(c.ends_at)}`;
  }
  return `${formatDateTime(c.clock_started_at ?? c.starts_at)} 〜 ${formatDateTime(c.ends_at)}`;
}

// sortTrialFirst は、お試しプランを先頭にする。ほかの並び（安い順）はそのまま。
export function sortTrialFirst<T extends Pick<CoachingPlan, "kind">>(plans: T[]): T[] {
  return [...plans.filter((p) => p.kind === "trial"), ...plans.filter((p) => p.kind !== "trial")];
}

// パーソナル指導の誓約の文面。API の coachPledgeVersion と一緒に変える。
export const coachPledgeItems = [
  "LINE や SNS の ID、電話番号、メールアドレスなど、TrackTalk の外の連絡先を教えたり、聞いたりしません。",
  "利用者と直接会うのは、TrackTalk で購入・予約された対面指導だけにします。それ以外の直接の約束や、TrackTalk を通さないお金のやりとりはしません。",
  "連絡先のやりとりを防ぐため、送る文章を機械で確かめ、連絡先らしいものは送れなくなることに同意します（添削の返答も同じです）。",
  "通報があったときは、安全の確認のために運営がやりとりを読むことに同意します。",
  "守らなかったときは、利用停止と報酬の没収になることに同意します。",
];

const contractStatusLabels: Record<CoachingContractStatus, string> = {
  applied: "コーチの返事待ち",
  accepted: "保護者の同意待ち",
  active: "期間中",
  suspended: "運営が止めています",
  ended: "終わりました",
  refunded: "返金して終了しました",
  declined: "今回は受けられませんでした",
  guardian_declined: "保護者が同意しませんでした",
  expired: "期限が過ぎました",
  cancelled: "取り下げました",
};

// contractStatusLabel は、支払いがあるかと年齢で変わる「承諾済み・始まる前」の表示を含めた状態の名前。
export function contractStatusLabel(c: { status: CoachingContractStatus; minor: boolean; payment_required: boolean }): string {
  if (c.status === "accepted") {
    if (c.minor && c.payment_required) return "保護者の同意とお支払い待ち";
    if (c.payment_required) return "お支払い待ち";
  }
  return contractStatusLabels[c.status];
}

// 契約を始めるときの同意。利用者は申し込みで、コーチは承諾で、それぞれ別にチェックする。
// 文面を変えたら API の contractConsentVersion を上げる。
export const contractConsentItems = {
  agree_report: "通報があったとき、安全の確認のために TrackTalk の運営がこのやりとりを読むことに同意します。普段は運営も読みません。",
  agree_detection:
    "外部の連絡先（URL・電話番号・SNS の ID など）を送ろうとしたとき、機械で検出して送信を止めることに同意します。",
} as const;
export type ContractConsentKey = keyof typeof contractConsentItems;
