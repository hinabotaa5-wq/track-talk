import type { CloudinaryUploadProgress } from "@/lib/cloudinary-upload";

function formatAmount(bytes: number, total: number) {
  if (total >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB / ${(total / (1024 * 1024)).toFixed(1)} MB`;
  }
  if (total >= 1024) {
    return `${Math.round(bytes / 1024)} KB / ${Math.round(total / 1024)} KB`;
  }
  return `${Math.round(bytes)} B / ${Math.round(total)} B`;
}

export function UploadProgress({
  noun,
  phase,
  progress,
}: {
  noun: "写真" | "動画";
  phase: "upload" | "saving";
  progress: CloudinaryUploadProgress;
}) {
  const sent = progress.total > 0 && progress.loaded >= progress.total;
  const shown = phase === "saving" || sent ? 100 : Math.min(99, Math.round(progress.percent));
  const label =
    phase === "saving" ? "投稿を保存しています" : sent ? `${noun}を送り終わりました` : `${noun}を送っています`;
  const detail =
    phase === "upload" && sent ? "Cloudinaryが取り込みを終えるまで、このまま待ってください。" : undefined;

  return (
    <div className="rounded-2xl border border-blue-200 bg-blue-50 p-4">
      <p className="text-sm font-medium text-zinc-800">{label}</p>
      <p className="mt-1 text-5xl font-bold leading-none tabular-nums text-blue-700">{shown}%</p>
      <div
        className="mt-3 h-4 overflow-hidden rounded-full bg-white"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={shown}
        aria-valuetext={`${shown}パーセント`}
        aria-label={label}
      >
        <div className="h-full rounded-full bg-blue-600 transition-[width] duration-150 ease-out" style={{ width: `${shown}%` }} />
      </div>
      <p className="mt-2 text-sm tabular-nums text-zinc-600">{formatAmount(progress.loaded, progress.total)}</p>
      {detail ? <p className="mt-1 text-sm text-zinc-600">{detail}</p> : null}
    </div>
  );
}
