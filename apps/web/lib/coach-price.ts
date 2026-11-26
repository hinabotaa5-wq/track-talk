import { L } from "@/lib/i18n";

// コーチの価格申請制（本体 #1052）。金額は円のみ。下限500円、上限なし。

export type CoachPricePlan = "standard" | "premium";

export type CoachPriceApplication = {
  id: string;
  plan: CoachPricePlan;
  price_yen: number;
  source: "price" | "coach_apply";
  status: "pending" | "approved" | "applied" | "withdrawn" | "superseded";
  created_at: string;
  updated_at: string;
  approved_at: string | null;
  applied_at: string | null;
};

export type CoachPricePlanState = {
  plan: CoachPricePlan;
  // いま有効な価格。プレミアムは未設定なら null。
  price_yen: number | null;
  // スタンダードで価格が未設定（共通の価格で受け付けている）か。
  is_default: boolean;
  pending: CoachPriceApplication | null;
  approved: CoachPriceApplication | null;
};

export type CoachPrices = { plans: CoachPricePlanState[]; min_price_yen: number; default_price_yen: number };

export type AdminCoachPriceApplication = {
  id: string;
  coach_id: string;
  coach_name: string;
  coach_status: string;
  coach_country_code: string;
  plan: CoachPricePlan;
  price_yen: number;
  current_price_yen: number | null;
  source: "price" | "coach_apply";
  status: "pending" | "approved" | "applied";
  created_at: string;
  updated_at: string;
  approved_at: string | null;
  approved_by_nickname: string | null;
  applied_at: string | null;
};

export const coachPriceMinYen = 500;

export const coachPricePlanLabels: Record<CoachPricePlan, string> = {
  get standard() { return L("スタンダード"); },
  get premium() { return L("プレミアム"); },
};

// 金額の入力を円の整数に直す。空欄は null、読めない値は NaN。全角数字・カンマ・「円」は許す。
export function parseYenInput(raw: string): number | null {
  const v = raw.replace(/[,，円¥￥\s]/g, "").replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0));
  if (v === "") return null;
  return /^\d+$/.test(v) ? Number(v) : NaN;
}
