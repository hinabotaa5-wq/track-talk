// API の nickname.go と同じ正規化と予約語。最終判定は API が行う。
const reservedWords = [
  "管理者", "管理人", "かんりしゃ", "カンリシャ", "かんりにん",
  "運営", "うんえい", "ウンエイ", "事務局", "じむきょく", "公式", "こうしき", "コウシキ",
  "スタッフ", "サポート", "モデレーター", "アドミン", "トラックトーク",
  "admin", "moderator", "staff", "support", "official", "system", "tracktalk",
];

export const reservedNicknameMessage = "このニックネームは使用できません。運営や公式を思わせる名前は使えません。";

export function isReservedNickname(value: string) {
  const normalized = value.normalize("NFKC").toLowerCase()
    .replace(/[\p{White_Space}\p{P}\p{Cf}\p{Variation_Selector}\u034f]/gu, "")
    .normalize("NFC")
    .replace(/サポーター|公式戦|supporters?/g, "");
  return reservedWords.some((word) => normalized.includes(word));
}
