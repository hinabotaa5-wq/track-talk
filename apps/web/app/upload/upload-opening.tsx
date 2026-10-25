import { LoaderCircle } from "lucide-react";

// 投稿画面が開くまで(ログイン状態の確認中も含む)は、＋ボタンと同じクルクルを出す。
export function UploadOpening() {
  return (
    <div role="status" aria-busy="true" className="flex min-h-[50svh] flex-col items-center justify-center gap-3 text-slate-200">
      <LoaderCircle className="size-8 animate-spin text-blue-300" aria-hidden />
      <p className="text-sm">投稿画面を開いています…</p>
    </div>
  );
}
