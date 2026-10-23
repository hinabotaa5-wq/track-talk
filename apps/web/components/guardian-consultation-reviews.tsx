"use client";
import { apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import type { CoachEvent } from "@/lib/types";
import Link from "next/link";
import { useEffect, useState } from "react";
export function GuardianConsultationReviews({ token }: { token: string }) {
  const [reviews, setReviews] = useState<{ id: string; event: CoachEvent; created_at: string }[]>([]);
  const [error, setError] = useState("");
  useEffect(() => { let live = true; apiFetch<{ reviews: typeof reviews }>(`/v1/guardian/${encodeURIComponent(token)}/consultation-reviews`).then((d) => { if (live) setReviews(d.reviews); }).catch(() => { if (live) setError("添削後の追加相談を読み込めませんでした"); }); return () => { live = false; }; }, [token]);
  if (!reviews.length && !error) return null;
  return <section className="space-y-2"><h2 className="font-bold">お子さんの添削後の追加相談</h2>{reviews.map((r) => <Link key={r.id} href={`/guardian/${encodeURIComponent(token)}/reviews/${r.id}`} className="block text-sm text-blue-300 underline">{coachEventLabels[r.event]} · {formatDateTime(r.created_at)}</Link>)}{error ? <p className="text-sm text-red-400">{error}</p> : null}</section>;
}
