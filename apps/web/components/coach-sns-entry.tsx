// コーチ詳細の SNS・ホームページ欄の1件分。生の URL 文字列をそのまま出さず、
// プラットフォーム名＋短いラベル（ハンドルやドメイン）のチップ風表示にする（本体 #643）。
// リンク先は変えない。URL でない入力（アカウント名等）は従来どおりテキストで出すが、見た目はチップに揃える。
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

export function CoachSnsEntry({ value }: { value: string }) {
  const label = snsLabel(value);
  if (!label) {
    return <span className={`${chipClass} text-slate-100`}>{value}</span>;
  }
  return (
    <a href={label.href} target="_blank" rel="noopener noreferrer" className={`${chipClass} text-blue-300 underline underline-offset-4`}>
      <span className="shrink-0 font-semibold no-underline">{label.platform}</span>
      <span className="min-w-0 truncate">{label.short}</span>
    </a>
  );
}
