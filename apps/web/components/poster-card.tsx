const posterThemes = [
  {
    gradient: "from-sky-500 via-blue-500 to-blue-700",
    lane: "01",
    tag: "SPRINT",
  },
  {
    gradient: "from-slate-700 via-sky-700 to-blue-600",
    lane: "04",
    tag: "STADIUM",
  },
  {
    gradient: "from-blue-500 via-sky-500 to-slate-700",
    lane: "02",
    tag: "VELOCITY",
  },
  {
    gradient: "from-sky-600 via-blue-600 to-slate-700",
    lane: "03",
    tag: "DISTANCE",
  },
  {
    gradient: "from-blue-600 via-slate-700 to-sky-600",
    lane: "05",
    tag: "FIELD",
  },
];

function pickTheme(id: string) {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash << 5) - hash + id.charCodeAt(i);
    hash |= 0;
  }
  return posterThemes[Math.abs(hash) % posterThemes.length];
}

export function PosterFallback({ id, title }: { id: string; title: string }) {
  const theme = pickTheme(id);

  return (
    <div
      className={`relative flex h-full w-full flex-col justify-between overflow-hidden bg-gradient-to-br ${theme.gradient} p-4 text-white select-none`}
    >
      {/* トラックレーンの幾何学ライン */}
      <svg
        className="pointer-events-none absolute -right-6 -bottom-6 size-40 text-white/10"
        viewBox="0 0 100 100"
        fill="none"
        aria-hidden="true"
      >
        <circle cx="50" cy="50" r="45" stroke="currentColor" strokeWidth="4" />
        <circle cx="50" cy="50" r="35" stroke="currentColor" strokeWidth="3" />
        <circle cx="50" cy="50" r="25" stroke="currentColor" strokeWidth="2" />
      </svg>

      {/* レーン番号のウォーターマーク */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute top-1 right-2 font-mono text-5xl font-black tracking-tighter text-white/15"
      >
        {theme.lane}
      </span>

      {/* 上部タグ */}
      <div className="flex items-center gap-1.5">
        <span className="size-1.5 rounded-full bg-white/80" />
        <span className="font-mono text-[10px] font-bold tracking-widest text-white/80">
          {theme.tag}
        </span>
      </div>

      {/* メイン見出し */}
      <p className="line-clamp-3 text-sm font-bold leading-snug drop-shadow-xs sm:text-base">
        {title}
      </p>

      {/* 下部の微小タイポ */}
      <div className="flex items-center justify-between text-[10px] font-medium text-white/70">
        <span>TRACK TALK</span>
        <span className="font-mono">TALK &gt;&gt;</span>
      </div>
    </div>
  );
}
