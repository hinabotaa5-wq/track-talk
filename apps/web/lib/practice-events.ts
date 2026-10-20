import { coachEventLabels } from "./coach-recruit";
import type { CoachEvent } from "./types";

// アプリ内の種目は、コーチの担当種目(coachEventLabels)に統一する。
// リレーは、コーチの担当種目と練習相手募集だけ。プロフィール・感覚メモ・競技会の記録・レビュー・ケガには入れない。
const relay: CoachEvent = "relay";

type EventOption = { value: CoachEvent; label: string };

// 練習相手募集とコーチで選べる種目(リレーを含む)。
export const practiceEvents: readonly EventOption[] = (Object.keys(coachEventLabels) as CoachEvent[]).map((value) => ({
  value,
  label: coachEventLabels[value],
}));

// 利用者の種目・感覚メモ・競技会の記録・レビュー・ケガで選べる種目。
export const appEvents: readonly EventOption[] = practiceEvents.filter((item) => item.value !== relay);

export const practiceEventValues = practiceEvents.map((event) => event.value) as [CoachEvent, ...CoachEvent[]];
export const appEventValues = appEvents.map((event) => event.value) as [CoachEvent, ...CoachEvent[]];
