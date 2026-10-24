import Link from "next/link";
import { ArrowRight, ClipboardCheck } from "lucide-react";

export function CoachApplyLink() {
  return (
    <Link href="/coach/apply" className="group flex min-h-16 items-center gap-3 rounded-2xl border border-blue-300/40 bg-blue-600 px-4 py-4 font-semibold text-white shadow-sm transition hover:bg-blue-500 active:scale-[0.98]">
      <ClipboardCheck className="size-6 shrink-0" aria-hidden />
      <span className="flex-1">コーチに応募する</span>
      <ArrowRight className="size-5 shrink-0 transition group-hover:translate-x-1" aria-hidden />
    </Link>
  );
}
