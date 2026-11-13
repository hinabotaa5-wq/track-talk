import type { PublicCoach } from "./types";

type CoachDetailPreview =
  Pick<PublicCoach, "id" | "nickname" | "photo_url" | "events"> &
  Partial<Pick<PublicCoach, "rating_count" | "rating_average" | "best_record" | "achievements" | "service_types" | "sns_urls" | "hitokoto" | "plans" | "support_consult">>;

const previewLifetimeMs = 60_000;
let preview: { coach: CoachDetailPreview; fetchedAt: number } | undefined;

// Keep only the selected coach in this tab's memory for client-side navigation.
export function rememberCoachDetailPreview(coach: CoachDetailPreview, fetchedAt: number) {
  preview = { coach, fetchedAt };
}

export function getCoachDetailPreview(id: string): CoachDetailPreview | undefined {
  if (preview?.coach.id === id && Date.now() - preview.fetchedAt < previewLifetimeMs) {
    return preview.coach;
  }
  return undefined;
}
