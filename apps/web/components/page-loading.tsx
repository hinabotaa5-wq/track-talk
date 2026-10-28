import { LoadingStopwatch } from "./loading-stopwatch";

export function PageLoading({ label = "読み込み中..." }: { label?: string }) {
  return (
    <div role="status" aria-busy="true" className="flex min-h-32 flex-col items-center justify-center gap-3 py-8 text-slate-200">
      <LoadingStopwatch />
      <p className="text-sm">{label}</p>
    </div>
  );
}
