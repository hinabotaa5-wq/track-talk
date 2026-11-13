"use client";
import { L } from "@/lib/i18n";

// ヘッダー右側の Instagram への外部リンク（本体 #629）。
// lucide-react に Instagram が無いため、線画の描き方を他アイコンに揃えたインライン SVG を使う
// （24x24・stroke 2・round。単色で currentColor のまま、グラデーションなし）。
export const instagramProfileUrl = "https://www.instagram.com/tracktalk.office/";

export function InstagramLink() {
  return (
    <a
      href={instagramProfileUrl}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={L("Instagram（@tracktalk.office）")}
      className="inline-flex size-11 shrink-0 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 active:scale-[0.98]"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
        <rect width="20" height="20" x="2" y="2" rx="5" ry="5" />
        <path d="M16 11.37A4 4 0 1 1 12.63 8 4 4 0 0 1 16 11.37z" />
        <line x1="17.5" x2="17.51" y1="6.5" y2="6.5" />
      </svg>
    </a>
  );
}
