import { Globe } from "lucide-react";

// コーチ詳細の SNS・ホームページ欄の1件分。URL は文字を出さずアイコンだけのリンクにする（本体 #712）。
// 行き先は aria-label／title（プラットフォーム名＋短いラベル）で伝える。リンク先は変えない。
// プロジェクトの lucide-react にはブランドアイコンが無いため、判定できる7プラットフォーム（Instagram／YouTube／
// X／TikTok／Facebook／LINE／note）はブランド色のグリフをインライン SVG で描く（#951）。判定できない URL は汎用の
// Globe（色は text-slate-200 のまま）。アイコンはどれも size-7（28px）、タップ領域は size-11（44px）。
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

// ブランドグリフ。色は各プラットフォームのブランド色で固定する（#951 §2）。どれも viewBox 24x24、aria-hidden。
const glyphClass = "size-7";
const INK = "#0F172A";
const WHITE = "#FFFFFF";

// Instagram の線画グリフ。形は instagram-link.tsx と同じ値（rect／path／line、viewBox・stroke 各値）で、
// 線の色だけブランド色 #E1306C にしている。
function InstagramGlyph() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="#E1306C"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      data-sns-icon="Instagram"
      className={glyphClass}
    >
      <rect width="20" height="20" x="2" y="2" rx="5" ry="5" />
      <path d="M16 11.37A4 4 0 1 1 12.63 8 4 4 0 0 1 16 11.37z" />
      <line x1="17.5" x2="17.51" y1="6.5" y2="6.5" />
    </svg>
  );
}

// YouTube：赤い角丸長方形＋中央の白い再生三角。
function YouTubeGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" data-sns-icon="YouTube" className={glyphClass}>
      <rect x="1" y="4.5" width="22" height="15" rx="4" ry="4" fill="#FF0000" />
      <path d="M9.75 8.6v6.8L15.6 12z" fill={WHITE} />
    </svg>
  );
}

// X：太い交差の字形。
function XGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" data-sns-icon="X" className={glyphClass}>
      <path
        fill={INK}
        d="M18.9 1.15h3.68l-8.04 9.19L24 22.85h-7.41l-5.8-7.59-6.63 7.59H.47l8.6-9.83L0 1.15h7.59l5.24 6.93zM17.61 20.64h2.04L6.49 3.24H4.3z"
      />
    </svg>
  );
}

// TikTok：音符形のブランドグリフ。
function TikTokGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" data-sns-icon="TikTok" className={glyphClass}>
      <path
        fill={INK}
        d="M12.53.02C13.84 0 15.14.01 16.44 0c.08 1.53.63 3.09 1.75 4.17 1.12 1.11 2.7 1.62 4.24 1.79v4.03c-1.44-.05-2.89-.35-4.2-.97-.57-.26-1.1-.59-1.62-.93-.01 2.92.01 5.84-.02 8.75-.08 1.4-.54 2.79-1.35 3.94-1.31 1.92-3.58 3.17-5.91 3.21-1.43.08-2.86-.31-4.08-1.03-2.02-1.19-3.44-3.37-3.65-5.71-.02-.5-.03-1-.01-1.49.18-1.9 1.12-3.72 2.58-4.96 1.66-1.44 3.98-2.13 6.15-1.72.02 1.48-.04 2.96-.04 4.44-.99-.32-2.15-.23-3.02.37-.63.41-1.11 1.04-1.36 1.75-.21.51-.15 1.07-.14 1.61.24 1.64 1.82 3.02 3.5 2.87 1.12-.01 2.19-.66 2.77-1.61.19-.33.4-.67.41-1.06.1-1.79.06-3.57.07-5.36.01-4.03-.01-8.05.02-12.07z"
      />
    </svg>
  );
}

// Facebook：「f」の字形。
function FacebookGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" data-sns-icon="Facebook" className={glyphClass}>
      <path
        fill="#1877F2"
        d="M13.6 23.5v-9.6h3.25l.5-3.8H13.6V7.7c0-1.1.31-1.85 1.88-1.85h2V2.45c-.35-.05-1.54-.15-2.92-.15-2.9 0-4.88 1.77-4.88 5.02v2.8H6.4v3.8h3.28v9.6z"
      />
    </svg>
  );
}

// LINE：緑の吹き出しに、白抜きの「LINE」の字形（<text> でなくパスで描く）。
function LineGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" data-sns-icon="LINE" className={glyphClass}>
      <path
        fill="#06C755"
        d="M12 1.8C6.2 1.8 1.5 5.65 1.5 10.4c0 4.25 3.72 7.8 8.75 8.48.34.07.8.22.92.51.1.26.07.67.03.94l-.15.9c-.04.27-.21 1.04.91.57 1.12-.47 6.04-3.56 8.24-6.09C21.72 14.05 22.5 12.3 22.5 10.4 22.5 5.65 17.8 1.8 12 1.8z"
      />
      <path
        fill={WHITE}
        d="M4.6 7.4h1.25v4.55H8.3v1.15H4.6zM9 7.4h1.25v5.7H9zM11.35 7.4h1.15l2.4 3.6V7.4h1.2v5.7h-1.15l-2.4-3.6v3.6h-1.2zM17.05 7.4h3.5v1.1H18.3v1.2h2.15v1.1H18.3V12h2.25v1.1h-3.5z"
      />
    </svg>
  );
}

// note：黒い角丸正方形に白抜きの「n」の字形。
function NoteGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" data-sns-icon="note" className={glyphClass}>
      <rect x="2" y="2" width="20" height="20" rx="4.5" ry="4.5" fill={INK} />
      <path fill={WHITE} d="M7 17V7.25h5.6A4 4 0 0 1 16.6 11.25V17h-2.5v-5.6c0-.95-.6-1.65-1.6-1.65H9.5V17z" />
    </svg>
  );
}

const brandGlyphs: Record<string, () => React.JSX.Element> = {
  Instagram: InstagramGlyph,
  YouTube: YouTubeGlyph,
  X: XGlyph,
  TikTok: TikTokGlyph,
  Facebook: FacebookGlyph,
  LINE: LineGlyph,
  note: NoteGlyph,
};

export function CoachSnsEntry({ value }: { value: string }) {
  const label = snsLabel(value);
  if (!label) {
    return <span className={`${chipClass} text-slate-100`}>{value}</span>;
  }
  const accessibleName = `${label.platform} ${label.short}`;
  const BrandGlyph = Object.hasOwn(brandGlyphs, label.platform) ? brandGlyphs[label.platform] : null;
  return (
    <a
      href={label.href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={accessibleName}
      title={accessibleName}
      className="inline-flex size-11 items-center justify-center rounded-full text-slate-200 transition hover:bg-white/10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-400 active:scale-95"
    >
      {BrandGlyph ? <BrandGlyph /> : <Globe className="size-7" aria-hidden="true" />}
    </a>
  );
}
