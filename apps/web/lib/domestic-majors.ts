import { apiFetch } from "./api";
import type { DomesticMajor } from "./types";

export type DomesticCategory = DomesticMajor["category"];

export type DomesticMeet = { slug: string; label: string; category: DomesticCategory };

// Every meet the importer looks for. A meet with no edition in the season is listed as 日程未定.
const DOMESTIC_MEETS: DomesticMeet[] = [
  { slug: "nissin-cup", label: "日清食品カップ 全国小学生陸上競技交流大会", category: "youth" },
  { slug: "zenchu", label: "全日本中学校陸上競技選手権大会", category: "youth" },
  { slug: "interhigh", label: "全国高等学校陸上競技対校選手権大会(インターハイ)", category: "youth" },
  { slug: "junior-u16-u18", label: "ジュニアオリンピック U16・U18", category: "youth" },
  { slug: "junior-u20", label: "ジュニアオリンピック U20", category: "youth" },
  { slug: "incare-hokkaido", label: "北海道学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-tohoku", label: "東北学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-kanto", label: "関東学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-hokushinetsu", label: "北信越学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-tokai", label: "東海学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-kansai", label: "関西学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-chushikoku", label: "中国四国学生陸上競技対校選手権大会", category: "student" },
  { slug: "incare-kyushu", label: "九州学生陸上競技対校選手権大会", category: "student" },
  { slug: "nihon-incare", label: "日本学生陸上競技対校選手権大会(全日本インカレ)", category: "student" },
  { slug: "nihon-championships", label: "日本陸上競技選手権大会", category: "open" },
  { slug: "jitsugyodan", label: "全日本実業団対抗陸上競技選手権大会", category: "open" },
  { slug: "izumo-ekiden", label: "出雲全日本大学選抜駅伝競走", category: "distance" },
  { slug: "univ-women-ekiden", label: "全日本大学女子駅伝対校選手権大会", category: "distance" },
  { slug: "univ-ekiden", label: "全日本大学駅伝対校選手権大会", category: "distance" },
  { slug: "queens-ekiden", label: "全日本実業団対抗女子駅伝競走大会(クイーンズ駅伝)", category: "distance" },
  { slug: "junior-high-ekiden", label: "全国中学駅伝大会", category: "distance" },
  { slug: "high-school-ekiden", label: "全国高等学校駅伝競走大会", category: "distance" },
  { slug: "fukuoka-marathon", label: "福岡国際マラソン", category: "distance" },
  { slug: "fujisan-ekiden", label: "全日本大学女子選抜駅伝競走(富士山女子駅伝)", category: "distance" },
  { slug: "new-year-ekiden", label: "全日本実業団対抗駅伝競走大会(ニューイヤー駅伝)", category: "distance" },
  { slug: "prefectural-women-ekiden", label: "全国都道府県対抗女子駅伝競走大会", category: "distance" },
  { slug: "prefectural-men-ekiden", label: "全国都道府県対抗男子駅伝競走大会", category: "distance" },
  { slug: "osaka-women-marathon", label: "大阪国際女子マラソン", category: "distance" },
  { slug: "beppu-oita-marathon", label: "別府大分毎日マラソン", category: "distance" },
  { slug: "osaka-marathon", label: "大阪マラソン", category: "distance" },
  { slug: "tokyo-marathon", label: "東京マラソン", category: "distance" },
  { slug: "nagoya-women-marathon", label: "名古屋ウィメンズマラソン", category: "distance" },
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
