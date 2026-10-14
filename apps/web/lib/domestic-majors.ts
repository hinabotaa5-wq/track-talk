import { apiFetch } from "./api";
import type { DomesticMajor } from "./types";

export type DomesticCategory = DomesticMajor["category"];

export type DomesticMeetCard = {
  slug: string;
  label: string;
  category: DomesticCategory;
  major: DomesticMajor | null;
};

export const DOMESTIC_CATEGORIES: { key: DomesticCategory; title: string }[] = [
  { key: "youth", title: "小・中・高" },
  { key: "student", title: "学生" },
  { key: "open", title: "日本選手権・社会人" },
];

// The cards stay in this order even when the API has no edition for a meet yet.
const DOMESTIC_MEETS: { slug: string; label: string; category: DomesticCategory }[] = [
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
];

export function domesticMeetCards(majors: DomesticMajor[]): DomesticMeetCard[] {
  const bySlug = new Map(majors.map((major) => [major.slug, major]));
  return DOMESTIC_MEETS.map((meet) => ({ ...meet, major: bySlug.get(meet.slug) ?? null }));
}

export function domesticMeetsOf(cards: DomesticMeetCard[], category: DomesticCategory) {
  return cards.filter((card) => card.category === category);
}

// today is a local "YYYY-MM-DD" string, the same shape as ends_on.
export function isDomesticMajorOver(major: DomesticMajor, today: string) {
  return major.ends_on.slice(0, 10) < today;
}

let cachedMajors: DomesticMajor[] | null = null;
let majorsRequest: Promise<DomesticMajor[]> | null = null;

export function peekDomesticMajors() {
  return cachedMajors;
}

export function clearDomesticMajors() {
  cachedMajors = null;
  majorsRequest = null;
}

export function loadDomesticMajors(token: string) {
  if (cachedMajors) return Promise.resolve(cachedMajors);
  if (!majorsRequest) {
    majorsRequest = apiFetch<{ majors: DomesticMajor[] }>("/v1/domestic-majors", { token })
      .then((data) => {
        cachedMajors = data.majors ?? [];
        return cachedMajors;
      })
      .catch((err) => {
        majorsRequest = null;
        throw err;
      });
  }
  return majorsRequest;
}
