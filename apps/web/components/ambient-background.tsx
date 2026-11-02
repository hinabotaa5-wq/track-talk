// 背景にロゴの青をごく薄くにじませ、上に重なるすりガラスのカードに透明感を出す。
// 色はロゴの青だけ(globals.css の --tt-blue-*)。明るい表示ではほとんど白に見える濃さにする。
// blur フィルタの大きな層は、iOS Safari で外部ページから戻ると破棄されて真っ暗になるため、radial-gradient で描く。
export function AmbientBackground() {
  return (
    <div aria-hidden="true" className="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
      <div className="absolute -top-48 -left-40 h-[620px] w-[620px] rounded-full bg-[radial-gradient(closest-side,color-mix(in_oklab,var(--tt-blue-500)_var(--ambient-strength),transparent),transparent)]" />
      <div className="absolute top-1/3 -right-48 h-[680px] w-[680px] rounded-full bg-[radial-gradient(closest-side,color-mix(in_oklab,var(--tt-blue-400)_var(--ambient-strength),transparent),transparent)]" />
    </div>
  );
}
