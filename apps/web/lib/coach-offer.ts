import { L } from "@/lib/i18n";

// コーチの添削項目（追加項目）と見本動画の変更申請（本体 #1086）。対象はスタンダードだけ。
// 固定の4項目（回答の型 #1067）は全コーチ共通でコーチは減らせない。追加項目（最大5件・各30字）と、
// コーチ詳細に常設する見本動画（最大3本）だけを申請で増減する。承認しても、承認の時点で対応中だった依頼が
// すべて終わるまでは切り替わらない（価格申請 #1052 と同じ）。

export const maxCoachOfferItems = 5;
export const maxCoachOfferItemChars = 30;
export const maxCoachDemoVideos = 3;

// 固定の4項目の正式名称（日本語）。API の fixedOfferTitles と同じ並び。追加項目のタイトルにはできない。
export const fixedOfferTitlesJa = ["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか"] as const;
// 固定4項目目の言い換え（旧入力例）。これも追加項目のタイトルにはできない（Issue 補足2 T2）。
export const reservedOfferTitlesJa: readonly string[] = [...fixedOfferTitlesJa, "コーチだったら明日から何をするか"];

// 表示用の固定4項目（回答表示の訳を使う）。
export function fixedOfferTitles(): string[] {
  return [L("良い点"), L("改善点"), L("おすすめのドリル"), L("コーチなら明日から何をするか")];
}

// 「このコーチの添削で得られるもの」の並び：固定4項目→追加項目（登録順、原文のまま）。
export function offerTitles(items: readonly string[] | null | undefined): string[] {
  return [...fixedOfferTitles(), ...(items ?? [])];
}

export type CoachDemoVideo = { url: string; thumbnail_url?: string; public_id?: string };

export type CoachOfferApplication = {
  id: string;
  items: string[];
  demo_videos: CoachDemoVideo[];
  status: "pending" | "approved" | "applied" | "withdrawn" | "superseded";
  created_at: string;
  updated_at: string;
  approved_at: string | null;
  applied_at: string | null;
};

export type CoachOffers = {
  items: string[];
  demo_videos: CoachDemoVideo[];
  pending: CoachOfferApplication | null;
  approved: CoachOfferApplication | null;
  max_items: number;
  max_item_chars: number;
  max_demo_videos: number;
};

export type AdminCoachOfferApplication = {
  id: string;
  coach_id: string;
  coach_name: string;
  coach_status: string;
  items: string[];
  demo_videos: CoachDemoVideo[];
  current_items: string[];
  current_demo_videos: CoachDemoVideo[];
  status: "pending" | "approved" | "applied";
  created_at: string;
  updated_at: string;
  approved_at: string | null;
  approved_by_nickname: string | null;
  applied_at: string | null;
};

// 追加項目の入力チェック（API と同じ規則）。問題がなければ空文字。
export function coachOfferItemsError(items: readonly string[]): string {
  if (items.length > maxCoachOfferItems) return L("追加項目は5件までです");
  const seen = new Set<string>();
  for (const raw of items) {
    const title = raw.trim();
    if (!title) return L("追加項目のタイトルを入力してください");
    if (Array.from(title).length > maxCoachOfferItemChars) return L("追加項目のタイトルは30文字以内で入力してください");
    if (reservedOfferTitlesJa.includes(title)) return L("「{v0}」はすべての回答に入っている項目なので、追加項目にはできません", { v0: title });
    if (seen.has(title)) return L("同じタイトルの追加項目が重なっています");
    seen.add(title);
  }
  return "";
}
