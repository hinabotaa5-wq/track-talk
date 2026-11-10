import { L } from "./i18n";
import type { CoachEvent, CoachStatus, CoachTier } from "./types";

// コーチ募集の応募画面。
export const coachApplyPath = "/coach/apply";

export const coachTierLabels: Record<CoachTier, string> = {
  get coach() { return L("コーチ"); },
};

// 担当種目。並びは画面に出す順。
export const coachEventLabels: Record<CoachEvent, string> = {
  "100m": "100m",
  "200m": "200m",
  "400m": "400m",
  "800m": "800m",
  "1500m": "1500m",
  "5000m": "5000m",
  "10000m": "10000m",
  get "110mh"() { return L("110mハードル"); },
  get "100mh"() { return L("100mハードル"); },
  get "400mh"() { return L("400mハードル"); },
  get "3000msc"() { return L("3000m障害物"); },
  get "high-jump"() { return L("走高跳"); },
  get "pole-vault"() { return L("棒高跳"); },
  get "long-jump"() { return L("走幅跳"); },
  get "triple-jump"() { return L("三段跳"); },
  get "shot-put"() { return L("砲丸投"); },
  get "discus"() { return L("円盤投"); },
  get "hammer"() { return L("ハンマー投"); },
  get "javelin"() { return L("やり投"); },
  get "race-walk"() { return L("競歩"); },
  get "relay"() { return L("リレー"); },
  get "heptathlon"() { return L("七種競技"); },
  get "decathlon"() { return L("十種競技"); },
};

export const coachEvents = Object.keys(coachEventLabels) as CoachEvent[];

export const coachStatusLabels: Record<CoachStatus, string> = {
  get pending() { return L("審査中"); },
  get approved() { return L("承認済み"); },
  get rejected() { return L("見送り"); },
  get suspended() { return L("停止中"); },
  get withdrawn() { return L("やめた"); },
  get ended() { return L("登録終了"); },
};
