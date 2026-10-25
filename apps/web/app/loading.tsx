export default function Loading() {
  return (
    <div role="status" aria-busy="true" className="min-h-svh space-y-4 md:min-h-0">
      <p className="sr-only">画面を準備しています</p>
      <div aria-hidden className="h-10 w-1/2 animate-pulse rounded-xl bg-white/12" />
      <div aria-hidden className="h-32 animate-pulse rounded-2xl bg-white/12" />
      <div aria-hidden className="h-32 animate-pulse rounded-2xl bg-white/12" />
    </div>
  );
}
