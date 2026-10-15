"use client";

const colors = ["#EF9F27", "#D4537E", "#378ADD", "#1D9E75", "#7F77DD", "#D85A30"];

// 初めてゴールに着いたときの演出。紙吹雪は「動きを減らす」設定の人には出ない(CSS側で非表示)。
export function InjuryGoalCelebration({ onClose }: { onClose: () => void }) {
  return (
    <div role="status" className="relative overflow-hidden rounded-2xl border border-emerald-500/60 bg-emerald-950/50 p-5 text-center">
      {Array.from({ length: 28 }, (_, i) => (
        <span
          key={i}
          className="confetti-piece"
          style={{
            left: `${(i * 37) % 96 + 2}%`,
            backgroundColor: colors[i % colors.length],
            animationDelay: `${((i * 7) % 6) / 10}s`,
          }}
        />
      ))}
      <p className="text-lg font-bold text-emerald-200">大会に復帰、おめでとう</p>
      <p className="mt-1 text-sm text-emerald-100">あなたも乗り越えた仲間の一人です。あなたの経験は、後ろの仲間のヒントになります。</p>
      <button
        type="button"
        onClick={onClose}
        className="mt-3 rounded-full border border-emerald-400/60 px-4 py-1.5 text-sm font-semibold text-emerald-100 transition hover:bg-emerald-900/50 active:scale-95"
      >
        閉じる
      </button>
    </div>
  );
}
