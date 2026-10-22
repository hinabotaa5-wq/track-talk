export function SubmittingProgress({ label = "投稿を送信しています" }: { label?: string }) {
  return (
    <div
      className="rounded-2xl border border-blue-800/60 bg-blue-950/40 p-4"
      role="status"
      aria-live="polite"
    >
      <p className="text-sm font-semibold text-slate-100">{label}…</p>
      <div
        className="upload-indeterminate-track mt-3 h-4 overflow-hidden rounded-full glass bg-white/12"
        role="progressbar"
        aria-label={label}
      >
        <div className="upload-indeterminate h-full rounded-full bg-blue-600" />
      </div>
      <p className="mt-2 text-sm text-slate-200">このままお待ちください</p>
    </div>
  );
}
