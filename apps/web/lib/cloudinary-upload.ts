import { apiFetch } from "./api";

export type CloudinarySignature = {
  cloud_name: string;
  api_key: string;
  timestamp: number;
  public_id: string;
  signature: string;
  upload_preset: string;
  allowed_formats: string;
  resource_type: "image" | "video";
  // 添削の動画は非公開（authenticated）で上げる。署名に含まれるので、そのまま送る。
  type?: "authenticated";
  // 配信用の変換を Cloudinary に裏で作らせる指定（#494）。署名に含まれるので、そのまま送る。
  eager?: string;
  eager_async?: string;
};

// 動画の署名を取りにいくときに付ける。付けると、保存前に縮めず eager で変換を作る署名が返る。
// Cloudinary は大きな動画（Free は 40MB 超）を同期で変換できず、保存前に縮める方式では断られるため。
export const eagerVideoQuery = "?eager=1";

export type CloudinaryUploadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

type CloudinaryUploadResult = {
  public_id?: string;
  error?: { message?: string };
};

// 署名の期限切れ（Cloudinary は発行から1時間を過ぎた署名を断る）のときの文言。
export const staleUploadMessage = "アップロードの期限が切れました。もう一度試してください";

// CloudinaryUploadError は、利用者向けの文言（message）と、Cloudinary が返した生の文（detail）を両方持つ。
// 生の文には上限値などの手がかりが入るので、捨てずにコンソールとエラーの詳細に残す。
export class CloudinaryUploadError extends Error {
  detail: string;
  status: number;

  constructor(message: string, detail: string, status: number) {
    super(message);
    this.name = "CloudinaryUploadError";
    this.detail = detail;
    this.status = status;
  }
}

export function cloudinaryErrorMessage(message?: string): string {
  const text = message?.toLowerCase() ?? "";
  if (text.includes("file size") || text.includes("too large")) {
    return "ファイルサイズが大きすぎます";
  }
  if (text.includes("image")) return "写真ファイルを確認してください";
  if (text.includes("video") || text.includes("format")) return "対応していないファイル形式です";
  if (text.includes("stale") || text.includes("signature") || text.includes("timestamp")) {
    return staleUploadMessage;
  }
  return "アップロードに失敗しました";
}

export function uploadToCloudinary(
  resource: "image" | "video",
  file: File,
  signed: CloudinarySignature,
  onProgress: (progress: CloudinaryUploadProgress) => void,
  signal?: AbortSignal,
): Promise<CloudinaryUploadResult> {
  if (signed.resource_type !== resource) {
    return Promise.reject(new Error("アップロードに失敗しました"));
  }
  const body = new FormData();
  body.append("file", file);
  body.append("api_key", signed.api_key);
  body.append("timestamp", String(signed.timestamp));
  body.append("public_id", signed.public_id);
  body.append("upload_preset", signed.upload_preset);
  body.append("allowed_formats", signed.allowed_formats);
  body.append("signature", signed.signature);
  if (signed.type) body.append("type", signed.type);
  if (signed.eager) body.append("eager", signed.eager);
  if (signed.eager_async) body.append("eager_async", signed.eager_async);

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `https://api.cloudinary.com/v1_1/${signed.cloud_name}/${resource}/upload`);
    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable || event.total <= 0) return;
      onProgress({
        loaded: event.loaded,
        total: event.total,
        percent: (event.loaded / event.total) * 100,
      });
    };
    xhr.onerror = () => reject(new Error("アップロードに失敗しました"));
    // 選び直しなどで呼び出し側が止めたときは、失敗ではないことが分かるよう AbortError で返す。
    xhr.onabort = () =>
      reject(signal?.aborted ? new DOMException("Aborted", "AbortError") : new Error("アップロードに失敗しました"));
    xhr.onload = () => {
      let payload: CloudinaryUploadResult = {};
      try {
        payload = JSON.parse(xhr.responseText) as CloudinaryUploadResult;
      } catch {
        reject(new Error("アップロードに失敗しました"));
        return;
      }
      if (xhr.status < 200 || xhr.status >= 300) {
        const detail = payload.error?.message ?? "";
        console.error("Cloudinary upload failed", xhr.status, detail);
        reject(new CloudinaryUploadError(cloudinaryErrorMessage(detail), detail, xhr.status));
        return;
      }
      resolve(payload);
    };
    if (signal) {
      if (signal.aborted) {
        reject(new DOMException("Aborted", "AbortError"));
        return;
      }
      signal.addEventListener("abort", () => xhr.abort(), { once: true });
    }
    xhr.send(body);
  });
}

export const videoProcessingTimeoutMessage =
  "動画の準備に時間がかかっています。しばらくしてからもう一度試してください";

type VideoStatus = { ready: boolean };

// 待つ間隔（ミリ秒）。最初は短く、だんだん長くする。合計でおよそ10分待つ。
const videoStatusDelays = [2000, 3000, 4000, 5000, 7000, 10000];
const videoStatusTimeout = 10 * 60 * 1000;

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException("Aborted", "AbortError"));
    };
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

// waitForVideoReady は、上げ終えた動画の再生用・サムネイル用の変換ができるまで待つ。
// 大きな動画は Cloudinary が裏で変換するので、できる前に投稿すると再生できない投稿になる。小さい動画はすぐ返る。
export async function waitForVideoReady(
  publicId: string,
  token: string | null,
  signal?: AbortSignal,
  now: () => number = Date.now,
): Promise<void> {
  const started = now();
  for (let attempt = 0; ; attempt++) {
    const status = await apiFetch<VideoStatus>("/v1/uploads/video/status", {
      method: "POST",
      token,
      body: JSON.stringify({ public_id: publicId }),
      signal,
    });
    if (status.ready) return;
    if (now() - started >= videoStatusTimeout) throw new Error(videoProcessingTimeoutMessage);
    await sleep(videoStatusDelays[Math.min(attempt, videoStatusDelays.length - 1)], signal);
  }
}

// uploadVideoToCloudinary は、動画を上げてから、配信用の変換ができるまで待つ。
// 署名に eager が無い（古い API が出した）ときは、これまでどおり上げるだけ。
export async function uploadVideoToCloudinary(
  file: File,
  signed: CloudinarySignature,
  token: string | null,
  onProgress: (progress: CloudinaryUploadProgress) => void,
  signal?: AbortSignal,
): Promise<CloudinaryUploadResult> {
  const payload = await uploadToCloudinary("video", file, signed, onProgress, signal);
  if (signed.eager && payload.public_id) {
    await waitForVideoReady(payload.public_id, token, signal);
  }
  return payload;
}
