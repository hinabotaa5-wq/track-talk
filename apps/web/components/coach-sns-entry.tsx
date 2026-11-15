import { Globe } from "lucide-react";

// コーチ詳細の SNS・ホームページ欄の1件分。URL は文字を出さずアイコンだけのリンクにする（本体 #712）。
// 行き先は aria-label／title（プラットフォーム名＋短いラベル）で伝える。リンク先は変えない。
// プロジェクトの lucide-react にはブランドアイコンが無いため、Instagram は instagram-link.tsx と同じ
// 線画グリフ（24x24・stroke 2・round・currentColor）のインライン SVG を使い、それ以外のプラットフォームでは
// 汎用の Globe を使う。
// URL でない入力（アカウント名等）は従来どおりテキストで出すが、見た目はチップに揃える。
const platformPatterns: [RegExp, string][] = [
  [/^(www\.|m\.)?instagram\.com$/, "Instagram"],
  [/^(www\.)?x\.com$/, "X"],
  [/^(www\.)?twitter\.com$/, "X"],
  [/^(www\.|m\.)?youtube\.com$/, "YouTube"],
  [/^youtu\.be$/, "YouTube"],
  [/^(www\.)?tiktok\.com$/, "TikTok"],
  [/^(www\.)?facebook\.com$/, "Facebook"],
  [/^line\.me$/, "LINE"],
  [/^(www\.)?note\.com$/, "note"],
];

export function snsLabel(value: string): { platform: string; short: string; href: string } | null {
  let url: URL;
  try {
    url = new URL(value.trim());
  } catch {
    return null;
  }
  if ((url.protocol !== "http:" && url.protocol !== "https:") || url.username || url.password) return null;
  const host = url.hostname.toLowerCase().replace(/\.$/, "");
  const bare = host.replace(/^www\./, "");
  const platform = platformPatterns.find(([pattern]) => pattern.test(host))?.[1] ?? bare;
  const parts = url.pathname.split("/").filter(Boolean);
  // プロフィールのような1階層のパスはハンドルとして出す（@ を付けていなくても付ける）。
  const short = parts.length === 1 ? `@${parts[0].replace(/^@/, "")}` : bare;
  return { platform, short, href: url.href };
}

const chipClass =
  "inline-flex max-w-full items-center gap-1.5 rounded-full border border-white/20 bg-white/10 px-3 py-1 break-all";

// Instagram の線画グリフ。描き方は instagram-link.tsx と同じ値（rect／path／line、viewBox・stroke 各値）で、
// 大きさだけ他のアイコンに揃えて size-6（24px）にしている。
function InstagramGlyph() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className="size-6"
    >
      <rect width="20" height="20" x="2" y="2" rx="5" ry="5" />
      <path d="M16 11.37A4 4 0 1 1 12.63 8 4 4 0 0 1 16 11.37z" />
      <line x1="17.5" x2="17.51" y1="6.5" y2="6.5" />
    </svg>
  );
}

export function CoachSnsEntry({ value }: { value: string }) {
  const label = snsLabel(value);
  if (!label) {
    return <span className={`${chipClass} text-slate-100`}>{value}</span>;
  }
  const accessibleName = `${label.platform} ${label.short}`;
  return (
    <a
      href={label.href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={accessibleName}
      title={accessibleName}
      className="inline-flex size-11 items-center justify-center rounded-full text-slate-200 transition hover:bg-white/10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-400 active:scale-95"
    >
      {label.platform === "Instagram" ? <InstagramGlyph /> : <Globe className="size-6" aria-hidden="true" />}
    </a>
  );
}
