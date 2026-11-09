// 外部の連絡手段（SNS・LINE 等）への誘導に使われやすい言葉。見つけても送信は止めず、送る前に警告する（本体 #485）。
// 連絡先そのもの（URL・メール・電話・ID の形）は API が送信を止める。並びは API の contact.go の solicitationPatterns と同じ。

const patterns = [
  /\bline\b|らいん|ライン\s*(交換|教え|追加|やって|で話|でやり|で連絡|で送)|instagram|\binsta\b|インスタ|twitter|ツイッター|tiktok|ティックトック|discord|ディスコード|カカオ|kakao|facebook|フェイスブック|threads|スレッズ/i,
  /\bDM\b|ＤＭ|メアド|メールアドレス|電話番号|連絡先|直接(連絡|やりとり|やり取り|取引|指導|会)|個人レッスン|個別(レッスン|指導|に連絡)|(外|ほか|他)で(やりとり|やり取り|連絡|会)/i,
];

// 字形をそろえ、見えない文字を除く（全角英数・飾り文字は NFKC で半角へ）。
function normalize(s: string): string {
  return s.normalize("NFKC").replace(/[​-‏⁠﻿]/g, "");
}

export function looksLikeSolicitation(text: string): boolean {
  const t = normalize(text);
  return patterns.some((re) => re.test(t));
}

// 各入力欄の近くに常に出す注意文。
export const contactNoticeText =
  "TrackTalk の外の連絡手段（LINE・SNS など）へ誘うことや、連絡先の交換は規約で禁止されています。URL・メールアドレス・電話番号・SNS の ID は送れません。困ったときはやりとりから通報できます。";

export const solicitationWarningText =
  "外の連絡手段へ誘うように読める言葉が入っています。TrackTalk の外でのやりとりや直接の取引は規約で禁止されています。問題がなければ、もう一度押すとそのまま送ります。";
