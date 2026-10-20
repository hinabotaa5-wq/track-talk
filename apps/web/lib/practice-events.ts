import { coachEventLabels } from "./coach-recruit";
import type { CoachEvent } from "./types";

// アプリ内の種目は、すべてコーチの担当種目(coachEventLabels)と同じ一覧。
// コーチ・練習相手募集・利用者の種目(登録のときに選ぶ)・感覚メモ・競技会の記録・レビュー・ケガで使う。
export const appEvents: readonly { value: CoachEvent; label: string }[] = (Object.keys(coachEventLabels) as CoachEvent[]).map((value) => ({
  value,
  label: coachEventLabels[value],
}));

export const appEventValues = appEvents.map((event) => event.value) as [CoachEvent, ...CoachEvent[]];
