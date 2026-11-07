export function CoachSnsEntry({ value }: { value: string }) {
  let url: URL | undefined;
  try {
    url = new URL(value);
  } catch {
    // アカウント名など、URL 以外の入力もそのまま表示する。
  }

  if (url && (url.protocol === "http:" || url.protocol === "https:") && !url.username && !url.password) {
    return <a href={url.href} target="_blank" rel="noopener noreferrer" className="block break-all text-blue-300 underline underline-offset-4">{value}</a>;
  }
  return <span className="block whitespace-pre-wrap break-all">{value}</span>;
}
