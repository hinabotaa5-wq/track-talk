// 外部の連絡手段（SNS・LINE 等）への誘導に使われやすい言葉。見つけても送信は止めず、送る前に警告する（本体 #485）。
// 並びは API の contact.go の solicitationPatterns と同じ。

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

// 申込みの確認前にも、API の contact.go の looksLikeContact と同じ基準で判定する。
const contactPatterns = [
  /https?:\/\/|www\./i,
  /\b[a-z0-9-]+\.(com|net|org|jp|io|me|co|app|link|ly|gg|tv|ee|dev|xyz|info|page|site|blog)\b/i,
  /[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}/i,
  /@[a-z0-9_.]{3,}/i,
  /\bID[ \t\n\f\r]*[:：は]/i,
  /(\bline\b|ライン|らいん|instagram|\binsta\b|インスタ|twitter|\bx\b|tiktok|discord|kakao|カカオ|facebook|threads)[ \t\n\f\r]*(id|ｉｄ)?[ \t\n\f\r]*[:：][ \t\n\f\r]*[a-z0-9_.]{3,}/i,
  /(\bline\b|ライン|らいん)[ \t\n\f\r]*(id|ｉｄ)/i,
];
const kanjiDigits: Record<string, string> = {
  〇: "0", 零: "0", 一: "1", 二: "2", 三: "3", 四: "4", 五: "5", 六: "6", 七: "7", 八: "8", 九: "9",
};

function looksLikePhone(text: string): boolean {
  // 小数点で区切ったラップは電話番号として扱わない。
  for (const run of text.matchAll(/\d(?:[ \t\n\f\r\-\u2010-\u2015\u2212ー~〜()_/・]*\d)*/g)) {
    const groups = [...run[0].matchAll(/\d+/g)];
    if (groups.some(([group]) => group.length >= 10)) return true;
    if (groups.length >= 10 && groups[0][0] === "0" && groups.every(([group]) => group.length === 1)) return true;

    let prefix = 0;
    for (const [i, group] of groups.entries()) {
      if (i > 0) {
        const previous = groups[i - 1];
        prefix += previous[0].length;
        if (/[ \t\n〜~]/.test(run[0].slice(previous.index + previous[0].length, group.index))) prefix = 0;
      }
      // 日付の途中から始まる数字列を電話番号として扱わない。
      if (prefix > 3 || !/^(0|81)/.test(group[0])) continue;

      let digits = 0;
      let longGroup = false;
      for (const [part] of groups.slice(i, i + 4)) {
        digits += part.length;
        longGroup ||= part.length >= 4;
        if (digits >= 10 && digits <= 13 && longGroup) return true;
      }
    }
  }
  return false;
}

export function looksLikeContact(text: string): boolean {
  const folded = text.normalize("NFKC")
    .replace(/[\p{Cf}\p{Me}\p{Variation_Selector}]/gu, "")
    .replace(/[〇零一二三四五六七八九]/g, (digit) => kanjiDigits[digit]);
  return contactPatterns.some((re) => re.test(folded)) || looksLikePhone(folded);
}

// 各入力欄の近くに常に出す注意文。
export const contactNoticeText =
  "TrackTalk の外の連絡手段（LINE・SNS など）へ誘うことや、連絡先の交換は規約で禁止されています。URL・メールアドレス・電話番号・SNS の ID は送れません。困ったときはやりとりから通報できます。";

export const solicitationWarningText =
  "外の連絡手段へ誘うように読める言葉が入っています。TrackTalk の外でのやりとりや直接の取引は規約で禁止されています。問題がなければ、もう一度押すとそのまま送ります。";
