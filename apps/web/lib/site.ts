// 検索エンジンに渡す URL の基準。本番では NEXT_PUBLIC_SITE_URL に公開ドメインを入れる。
export const siteUrl = (process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000").replace(/\/+$/, "");

export const siteName = "TrackTalk";
export const siteDescription =
  "陸上競技の練習・ケガ・大会・シューズのことを匿名で相談できるコミュニティ。練習動画へのアドバイスや合同練習の募集もできます。";

// 本文を検索結果の説明文として使える長さにそろえる。
export function snippet(text: string | undefined, max = 120) {
  const flat = (text ?? "").replace(/\s+/g, " ").trim();
  return flat.length > max ? `${flat.slice(0, max - 1)}…` : flat;
}
