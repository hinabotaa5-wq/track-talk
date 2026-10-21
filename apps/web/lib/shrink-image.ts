// 表示は長辺1600pxまでなので、少し余裕を持たせて2048pxまで縮めてから送る。
const maxPhotoEdge = 2048;
const jpegQuality = 0.85;

export function fitWithin(width: number, height: number, maxEdge = maxPhotoEdge): { width: number; height: number } {
  const longest = Math.max(width, height);
  if (longest <= maxEdge || longest <= 0) return { width, height };
  const scale = maxEdge / longest;
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  };
}

// アニメーションGIFは1コマ目しか残らないので縮めない。
export function canShrink(file: File): boolean {
  return file.type !== "image/gif" && !/\.gif$/i.test(file.name);
}

function outputType(file: File): string {
  return file.type === "image/png" || file.type === "image/webp" ? "image/webp" : "image/jpeg";
}

function renamed(name: string, type: string): string {
  const ext = type === "image/webp" ? "webp" : type === "image/png" ? "png" : "jpg";
  const base = name.replace(/\.[^.]+$/, "") || "photo";
  return `${base}.${ext}`;
}

async function decode(file: File): Promise<ImageBitmap | null> {
  if (typeof createImageBitmap !== "function") return null;
  try {
    return await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    return null;
  }
}

// 写真を長辺2048pxまで縮めて画質0.85で書き出す。読めない形式(Safari以外のHEICなど)や
// 縮めても小さくならないときは元のファイルをそのまま返す。
export async function shrinkImage(file: File): Promise<File> {
  if (!canShrink(file) || typeof document === "undefined") return file;
  const bitmap = await decode(file);
  if (!bitmap) return file;
  try {
    const size = fitWithin(bitmap.width, bitmap.height);
    const canvas = document.createElement("canvas");
    canvas.width = size.width;
    canvas.height = size.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return file;
    const type = outputType(file);
    if (type === "image/jpeg") {
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, size.width, size.height);
    }
    ctx.drawImage(bitmap, 0, 0, size.width, size.height);
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, type, jpegQuality));
    if (!blob || blob.size >= file.size) return file;
    return new File([blob], renamed(file.name, blob.type), { type: blob.type, lastModified: file.lastModified });
  } finally {
    bitmap.close();
  }
}
