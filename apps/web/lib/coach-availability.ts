import type { CoachingServiceType } from "./types";

export type AvailabilityStatus = "available" | "consult" | "unavailable" | "";
export type AvailabilityDay = { date: string; status: AvailabilityStatus; note: string };
export type CoachAvailability = { today: string; days: AvailabilityDay[] };

export const availabilityLabels: Record<AvailabilityStatus, string> = {
  available: "○ 空きあり", consult: "△ 要相談", unavailable: "× 不可", "": "− 未設定",
};
export function hasAvailability(service: CoachingServiceType | null | undefined): boolean {
  return service === "in_person" || service === "competition_support";
}
export function tokyoDate(now = new Date()): string {
  return new Intl.DateTimeFormat("sv-SE", { timeZone: "Asia/Tokyo", year: "numeric", month: "2-digit", day: "2-digit" }).format(now);
}
export function availabilityDays(today: string, days: AvailabilityDay[]): AvailabilityDay[] {
  return Array.from({ length: 21 }, (_, i) => {
    const date = new Date(`${today}T00:00:00Z`);
    date.setUTCDate(date.getUTCDate() + i);
    const key = date.toISOString().slice(0, 10);
    return days.find((day) => day.date === key) ?? { date: key, status: "", note: "" };
  });
}
export function availabilityDateLabel(date: string): string {
  const day = new Date(`${date}T00:00:00Z`);
  return `${day.getUTCMonth() + 1}/${day.getUTCDate()}（${"日月火水木金土"[day.getUTCDay()]}）`;
}
