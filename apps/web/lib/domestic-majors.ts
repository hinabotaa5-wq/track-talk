import { L } from "./i18n";
import { apiFetch } from "./api";
import type { DomesticMajor } from "./types";

export type DomesticCategory = DomesticMajor["category"];

export type DomesticMeet = { slug: string; label: string; category: DomesticCategory };

// Every meet the importer looks for. A meet with no edition in the season is listed as 日程未定.
const DOMESTIC_MEETS: DomesticMeet[] = [
  { slug: "nissin-cup", get label() { return L("日清食品カップ 全国小学生陸上競技交流大会"); }, category: "youth" },
  { slug: "zenchu", get label() { return L("全日本中学校陸上競技選手権大会"); }, category: "youth" },
  { slug: "interhigh", get label() { return L("全国高等学校陸上競技対校選手権大会(インターハイ)"); }, category: "youth" },
  { slug: "junior-u16-u18", get label() { return L("ジュニアオリンピック U16・U18"); }, category: "youth" },
  { slug: "junior-u20", get label() { return L("ジュニアオリンピック U20"); }, category: "youth" },
  { slug: "incare-hokkaido", get label() { return L("北海道学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-tohoku", get label() { return L("東北学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-kanto", get label() { return L("関東学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-hokushinetsu", get label() { return L("北信越学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-tokai", get label() { return L("東海学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-kansai", get label() { return L("関西学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-chushikoku", get label() { return L("中国四国学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "incare-kyushu", get label() { return L("九州学生陸上競技対校選手権大会"); }, category: "student" },
  { slug: "nihon-incare", get label() { return L("日本学生陸上競技対校選手権大会(全日本インカレ)"); }, category: "student" },
  { slug: "nihon-championships", get label() { return L("日本陸上競技選手権大会"); }, category: "open" },
  { slug: "jitsugyodan", get label() { return L("全日本実業団対抗陸上競技選手権大会"); }, category: "open" },
  { slug: "izumo-ekiden", get label() { return L("出雲全日本大学選抜駅伝競走"); }, category: "distance" },
  { slug: "univ-women-ekiden", get label() { return L("全日本大学女子駅伝対校選手権大会"); }, category: "distance" },
  { slug: "univ-ekiden", get label() { return L("全日本大学駅伝対校選手権大会"); }, category: "distance" },
  { slug: "queens-ekiden", get label() { return L("全日本実業団対抗女子駅伝競走大会(クイーンズ駅伝)"); }, category: "distance" },
  { slug: "junior-high-ekiden", get label() { return L("全国中学駅伝大会"); }, category: "distance" },
  { slug: "high-school-ekiden", get label() { return L("全国高等学校駅伝競走大会"); }, category: "distance" },
  { slug: "fukuoka-marathon", get label() { return L("福岡国際マラソン"); }, category: "distance" },
  { slug: "fujisan-ekiden", get label() { return L("全日本大学女子選抜駅伝競走(富士山女子駅伝)"); }, category: "distance" },
  { slug: "new-year-ekiden", get label() { return L("全日本実業団対抗駅伝競走大会(ニューイヤー駅伝)"); }, category: "distance" },
  { slug: "prefectural-women-ekiden", get label() { return L("全国都道府県対抗女子駅伝競走大会"); }, category: "distance" },
  { slug: "prefectural-men-ekiden", get label() { return L("全国都道府県対抗男子駅伝競走大会"); }, category: "distance" },
  { slug: "osaka-women-marathon", get label() { return L("大阪国際女子マラソン"); }, category: "distance" },
  { slug: "beppu-oita-marathon", get label() { return L("別府大分毎日マラソン"); }, category: "distance" },
  { slug: "osaka-marathon", get label() { return L("大阪マラソン"); }, category: "distance" },
  { slug: "tokyo-marathon", get label() { return L("東京マラソン"); }, category: "distance" },
  { slug: "nagoya-women-marathon", get label() { return L("名古屋ウィメンズマラソン"); }, category: "distance" },
];

export function undecidedDomesticMeets(majors: DomesticMajor[]) {
  const scheduled = new Set(majors.map((major) => major.slug));
  return DOMESTIC_MEETS.filter((meet) => !scheduled.has(meet.slug));
}

let cachedMajors: DomesticMajor[] | null = null;
let majorsRequest: Promise<DomesticMajor[]> | null = null;
// 消したあとに、消す前に始めた読み込みが古い一覧を書き戻さないよう、消すたびに進める。
let generation = 0;

export function peekDomesticMajors() {
  return cachedMajors;
}

export function clearDomesticMajors() {
  generation++;
  cachedMajors = null;
  majorsRequest = null;
}

export function loadDomesticMajors() {
  if (cachedMajors) return Promise.resolve(cachedMajors);
  if (!majorsRequest) {
    const startedAt = generation;
    const request: Promise<DomesticMajor[]> = apiFetch<{ majors: DomesticMajor[] }>("/v1/domestic-majors")
      .then((data) => {
        const majors = data.majors ?? [];
        if (startedAt === generation) cachedMajors = majors;
        return majors;
      })
      .catch((err) => {
        if (majorsRequest === request) majorsRequest = null;
        throw err;
      });
    majorsRequest = request;
  }
  return majorsRequest;
}
