export function AmbientBackground() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none fixed inset-0 -z-10 overflow-hidden"
    >
      {/* 青グレーのダークテーマに合わせた、スタジアムの夜間オーラ。
          blur フィルタの大きな層は、iOS Safari で外部ページから戻ると破棄されて真っ暗になるため、
          radial-gradient で描く。 */}
      <div className="absolute -top-40 -left-40 h-[600px] w-[600px] rounded-full bg-[radial-gradient(closest-side,rgb(59_130_246/0.45),rgb(99_102_241/0.2)_55%,transparent)]" />
      <div className="absolute top-1/4 -right-40 h-[650px] w-[650px] rounded-full bg-[radial-gradient(closest-side,rgb(34_211_238/0.35),rgb(37_99_235/0.2)_55%,transparent)]" />
      <div className="absolute -bottom-40 left-1/4 h-[550px] w-[550px] rounded-full bg-[radial-gradient(closest-side,rgb(56_189_248/0.3),rgb(29_78_216/0.15)_55%,transparent)]" />
    </div>
  );
}
