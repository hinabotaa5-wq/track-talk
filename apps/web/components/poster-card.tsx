// 写真などがない投稿のカード。本文の冒頭があれば本文を、なければタイトルを出す。
export function PosterFallback({ title, excerpt }: { title: string; excerpt?: string }) {
  return (
    <div className="flex h-full w-full items-end bg-white/6 p-3.5">
      <p className={`[overflow-wrap:anywhere] text-sm leading-snug text-slate-100 ${excerpt ? "line-clamp-6" : "line-clamp-4 font-medium"}`}>{excerpt || title}</p>
    </div>
  );
}
