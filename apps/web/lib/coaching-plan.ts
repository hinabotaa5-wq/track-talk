import type { CoachingContractStatus, CoachingPlan, CoachingPlanKind } from "./types";

// パーソナル指導のプランの表示と入力のきまり。API（coaching_plan.go）の値と合わせる。

export const planKindLabels: Record<CoachingPlanKind, string> = {
  period: "期間プラン",
  single: "単発",
};

// 種類ごとに選べる日数。単発は7日だけ。
export const planDurations: Record<CoachingPlanKind, number[]> = {
  period: [14, 30],
  single: [7],
};

export const minPlanPriceYen = 2000;
// これを超える値段は、保存する前に打ち間違いでないか確かめる。
export const highPlanPriceYen = 30000;

export function planDurationLabel(kind: CoachingPlanKind, days: number): string {
  if (kind === "single") return `${days}日以内`;
  if (days === 14) return "2週間";
  if (days === 30) return "1か月";
  return `${days}日`;
}

export function formatPlanPrice(yen: number): string {
  return `${yen.toLocaleString("ja-JP")}円`;
}

export function planSummary(plan: Pick<CoachingPlan, "kind" | "duration_days" | "price_yen">): string {
  return `${planKindLabels[plan.kind]} · ${planDurationLabel(plan.kind, plan.duration_days)} · ${formatPlanPrice(plan.price_yen)}`;
}

// パーソナル指導の誓約の文面。API の coachPledgeVersion と一緒に変える。
export const coachPledgeItems = [
  "LINE や SNS の ID、電話番号、メールアドレスなど、TrackTalk の外の連絡先を教えたり、聞いたりしません。",
  "利用者と直接会う約束や、TrackTalk を通さないお金のやりとりをしません。",
  "連絡先のやりとりを防ぐため、送る文章を機械で確かめ、連絡先らしいものは送れなくなることに同意します（添削の返答も同じです）。",
  "通報があったときは、安全の確認のために運営がやりとりを読むことに同意します。",
  "守らなかったときは、利用停止と報酬の没収になることに同意します。",
];

export const contractStatusLabels: Record<CoachingContractStatus, string> = {
  applied: "コーチの返事待ち",
  accepted: "保護者の同意待ち",
  active: "期間中",
  suspended: "運営が止めています",
  ended: "終わりました",
  declined: "今回は受けられませんでした",
  guardian_declined: "保護者が同意しませんでした",
  expired: "返事がないまま期限が過ぎました",
  cancelled: "取り下げました",
};

// 契約を始めるときの同意。利用者は申し込みで、コーチは承諾で、それぞれ別にチェックする。
// 文面を変えたら API の contractConsentVersion を上げる。
export const contractConsentItems = {
  agree_report: "通報があったとき、安全の確認のために TrackTalk の運営がこのやりとりを読むことに同意します。普段は運営も読みません。",
  agree_detection:
    "外部の連絡先（URL・電話番号・SNS の ID など）を送ろうとしたとき、機械で検出して送信を止めることに同意します。",
} as const;
export type ContractConsentKey = keyof typeof contractConsentItems;
