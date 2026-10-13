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

function formatSize(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${Math.round(bytes)} B`;
}

export function UploadProgress({
  noun,
  phase,
  progress,
  fileName,
  fileSize,
}: {
  noun: "写真" | "動画";
  phase: "upload" | "saving";
  progress: CloudinaryUploadProgress;
  fileName?: string;
  fileSize?: number;
}) {
  const sent = progress.total > 0 && progress.loaded >= progress.total;
  const sending = phase === "upload" && !sent;
  const shown = Math.min(99, Math.round(progress.percent));
  const label = phase === "saving" ? "投稿を保存しています" : sending ? `${noun}を送っています` : "取り込み中";

  return (
    <div className="rounded-2xl border border-blue-800/60 bg-blue-950/40 p-4">
      <p className="text-sm font-medium text-slate-100">{label}</p>
      {fileName ? <p className="mt-1 truncate text-sm text-slate-300">{fileName}</p> : null}
      {fileSize != null ? <p className="mt-1 text-sm tabular-nums text-slate-200">{formatSize(fileSize)}</p> : null}
      {sending ? <p className="mt-1 text-5xl font-bold leading-none tabular-nums text-blue-400">{shown}%</p> : null}
      {sending ? (
        <div
          className="mt-3 h-4 overflow-hidden rounded-full glass bg-white/12"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={shown}
          aria-valuetext={`${shown}パーセント`}
          aria-label={label}
        >
          <div className="h-full rounded-full bg-blue-600 transition-[width] duration-150 ease-out" style={{ width: `${shown}%` }} />
        </div>
      ) : (
        <div className="upload-indeterminate-track mt-3 h-4 overflow-hidden rounded-full glass bg-white/12" role="progressbar" aria-label={label}>
          <div className="upload-indeterminate h-full rounded-full bg-blue-600" />
        </div>
      )}
      {sending ? <p className="mt-2 text-sm tabular-nums text-slate-200">{formatAmount(progress.loaded, progress.total)}</p> : null}
      <p className="mt-1 text-sm text-slate-200">このあと投稿ページに移動します</p>
    </div>
  );
}
