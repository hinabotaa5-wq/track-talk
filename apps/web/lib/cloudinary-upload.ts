export type CloudinarySignature = {
  cloud_name: string;
  api_key: string;
  timestamp: number;
  public_id: string;
  signature: string;
  upload_preset: string;
  allowed_formats: string;
  resource_type: "image" | "video";
};

export type CloudinaryUploadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

type CloudinaryUploadResult = {
  public_id?: string;
  error?: { message?: string };
};

export function cloudinaryErrorMessage(message?: string): string {
  const text = message?.toLowerCase() ?? "";
  if (text.includes("file size") || text.includes("too large")) {
    return "ファイルサイズが大きすぎます";
  }
  if (text.includes("image")) return "写真ファイルを確認してください";
  if (text.includes("video") || text.includes("format")) return "対応していないファイル形式です";
  if (text.includes("stale") || text.includes("signature") || text.includes("timestamp")) {
    return "アップロードの期限が切れました。もう一度試してください";
  }
  return "アップロードに失敗しました";
}

export function uploadToCloudinary(
  resource: "image" | "video",
  file: File,
  signed: CloudinarySignature,
  onProgress: (progress: CloudinaryUploadProgress) => void,
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
    xhr.onabort = () => reject(new Error("アップロードに失敗しました"));
    xhr.onload = () => {
      let payload: CloudinaryUploadResult = {};
      try {
        payload = JSON.parse(xhr.responseText) as CloudinaryUploadResult;
      } catch {
        reject(new Error("アップロードに失敗しました"));
        return;
      }
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new Error(cloudinaryErrorMessage(payload.error?.message)));
        return;
      }
      resolve(payload);
    };
    xhr.send(body);
  });
}
