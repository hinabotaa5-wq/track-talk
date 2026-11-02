import { LoadingStopwatch } from "./loading-stopwatch";

// screen を付けると、ヘッダーと下のバーを除いた画面の真ん中に出す(ページ全体を読み込むとき用。globals.css の .page-loading-screen)。
// 付けないと、置いた場所の中で真ん中に出す(カードや一覧の一部を読み込むとき用)。
export function PageLoading({ label = "読み込み中...", screen = false }: { label?: string; screen?: boolean }) {
  return (
    <div
      role="status"
      aria-busy="true"
      className={`${screen ? "page-loading-screen" : "min-h-32 py-8"} flex flex-col items-center justify-center gap-3 text-slate-200`}
    >
      <LoadingStopwatch />
      <p className="text-sm">{label}</p>
    </div>
  );
}
