export function AmbientBackground() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none fixed inset-0 -z-10 overflow-hidden"
    >
      {/* 青グレーのダークテーマに合わせた、スタジアムの夜間オーラ */}
      <div className="absolute -top-40 -left-40 h-[600px] w-[600px] rounded-full bg-gradient-to-br from-blue-500/45 via-indigo-500/25 to-transparent blur-[120px]" />
      <div className="absolute top-1/4 -right-40 h-[650px] w-[650px] rounded-full bg-gradient-to-bl from-cyan-400/35 via-blue-600/30 to-transparent blur-[130px]" />
      <div className="absolute -bottom-40 left-1/4 h-[550px] w-[550px] rounded-full bg-gradient-to-tr from-sky-400/30 via-blue-700/20 to-transparent blur-[120px]" />
    </div>
  );
}
