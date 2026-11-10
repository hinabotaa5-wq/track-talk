"use client";

// 規約類の本文は日本語のまま出す（運営者決定 2026-11-11、本体 #530）。英語表示のときだけ、日本語版が正であることを英語で添える。
import { useLocale } from "./locale-provider";

export function JapaneseOfficialNote({ subject = "page" }: { subject?: "page" | "section" }) {
  const locale = useLocale();
  if (locale !== "en") return null;
  return (
    <p lang="en" className="rounded-lg border border-white/20 bg-white/5 p-2 text-xs leading-relaxed text-slate-200">
      {subject === "page"
        ? "This page is provided in Japanese only. The Japanese text is the official version."
        : "The following terms are provided in Japanese only. The Japanese text is the official version."}
    </p>
  );
}
