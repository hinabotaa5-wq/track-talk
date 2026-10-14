export function PosterFallback({ title }: { title: string }) {
  return (
    <div className="flex h-full w-full items-end bg-white/6 p-3.5">
      <p className="line-clamp-4 text-sm font-medium leading-snug text-slate-100">{title}</p>
    </div>
  );
}
