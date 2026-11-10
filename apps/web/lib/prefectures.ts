import { L } from "@/lib/i18n";
import { todayInTokyo } from "./format";

export const PREFECTURES = [
  { code: "01", get name() { return L("北海道"); } },
  { code: "02", get name() { return L("青森県"); } },
  { code: "03", get name() { return L("岩手県"); } },
  { code: "04", get name() { return L("宮城県"); } },
  { code: "05", get name() { return L("秋田県"); } },
  { code: "06", get name() { return L("山形県"); } },
  { code: "07", get name() { return L("福島県"); } },
  { code: "08", get name() { return L("茨城県"); } },
  { code: "09", get name() { return L("栃木県"); } },
  { code: "10", get name() { return L("群馬県"); } },
  { code: "11", get name() { return L("埼玉県"); } },
  { code: "12", get name() { return L("千葉県"); } },
  { code: "13", get name() { return L("東京都"); } },
  { code: "14", get name() { return L("神奈川県"); } },
  { code: "15", get name() { return L("新潟県"); } },
  { code: "16", get name() { return L("富山県"); } },
  { code: "17", get name() { return L("石川県"); } },
  { code: "18", get name() { return L("福井県"); } },
  { code: "19", get name() { return L("山梨県"); } },
  { code: "20", get name() { return L("長野県"); } },
  { code: "21", get name() { return L("岐阜県"); } },
  { code: "22", get name() { return L("静岡県"); } },
  { code: "23", get name() { return L("愛知県"); } },
  { code: "24", get name() { return L("三重県"); } },
  { code: "25", get name() { return L("滋賀県"); } },
  { code: "26", get name() { return L("京都府"); } },
  { code: "27", get name() { return L("大阪府"); } },
  { code: "28", get name() { return L("兵庫県"); } },
  { code: "29", get name() { return L("奈良県"); } },
  { code: "30", get name() { return L("和歌山県"); } },
  { code: "31", get name() { return L("鳥取県"); } },
  { code: "32", get name() { return L("島根県"); } },
  { code: "33", get name() { return L("岡山県"); } },
  { code: "34", get name() { return L("広島県"); } },
  { code: "35", get name() { return L("山口県"); } },
  { code: "36", get name() { return L("徳島県"); } },
  { code: "37", get name() { return L("香川県"); } },
  { code: "38", get name() { return L("愛媛県"); } },
  { code: "39", get name() { return L("高知県"); } },
  { code: "40", get name() { return L("福岡県"); } },
  { code: "41", get name() { return L("佐賀県"); } },
  { code: "42", get name() { return L("長崎県"); } },
  { code: "43", get name() { return L("熊本県"); } },
  { code: "44", get name() { return L("大分県"); } },
  { code: "45", get name() { return L("宮崎県"); } },
  { code: "46", get name() { return L("鹿児島県"); } },
  { code: "47", get name() { return L("沖縄県"); } },
] as const;

export const INTERNATIONAL = "00";

export function prefectureCode(raw: string | undefined) {
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 0 || n > 47) return "";
  return String(n).padStart(2, "0");
}

export function monthNumber(raw: string | undefined) {
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 1 || n > 12) return 0;
  return n;
}

export function associationListLabel(code: string) {
  const prefecture = PREFECTURES.find((item) => item.code === code);
  const name = (prefecture?.name ?? "").replace(/[都府県]$/, "");
  return L("{name}陸協の競技会一覧", { name });
}

export function prefectureName(code: string) {
  return PREFECTURES.find((item) => item.code === code)?.name ?? code;
}

export function currentMonthInTokyo(now = new Date()) {
  return Number(todayInTokyo(now).slice(5, 7));
}
