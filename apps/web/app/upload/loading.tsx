export default function UploadLoading() {
  return <div role="status" aria-busy="true" className="space-y-3">
    <p className="text-sm text-slate-200">投稿画面を開いています…</p>
    <div className="grid gap-2" aria-hidden>
      {Array.from({ length: 5 }, (_, i) => <div key={i} className="h-24 animate-pulse rounded-2xl border border-white/20 bg-white/10" />)}
    </div>
  </div>;
}
