"use client";
import { CoachingConsultations } from "@/components/coaching-consultations";
import Link from "next/link";
import { useParams } from "next/navigation";
export default function GuardianReviewConsultations() {
  const { token, id } = useParams<{ token: string; id: string }>();
  return <div className="mx-auto max-w-md space-y-5"><h1 className="text-2xl font-bold">添削後の追加相談（保護者）</h1><CoachingConsultations rootID={id} kind="review" token={null} guardianToken={token} /><Link href={`/guardian/${encodeURIComponent(token)}`} className="text-sm text-blue-300 underline">保護者のページへ</Link></div>;
}
