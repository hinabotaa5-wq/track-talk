"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { uploadToCloudinary, type CloudinarySignature } from "@/lib/cloudinary-upload";
import { shrinkImage } from "@/lib/shrink-image";
import type { CoachApplication } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useState } from "react";

// コーチの顔写真。コーチ一覧・返答と一緒に届くプロフィール・コーチ画面で使う。
// 写真がなければ、ニックネームの1文字目を丸の中に出す。

const maxPhotoBytes = 10 * 1024 * 1024;

export function CoachAvatar({ url, name, size = 56 }: { url: string; name: string | null; size?: number }) {
  const style = { width: size, height: size };
  if (url) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={url} alt={`${name ?? "コーチ"}の顔写真`} style={style} className="shrink-0 rounded-full bg-white/10 object-cover" />;
  }
  return (
    <span
      aria-hidden
      style={style}
      className="flex shrink-0 items-center justify-center rounded-full bg-white/15 text-lg font-bold text-slate-200"
    >
      {(name ?? "?").slice(0, 1)}
    </span>
  );
}

// CoachPhotoForm は、応募中か承認済みのコーチが顔写真を選んで登録する。
export function CoachPhotoForm({
  application,
  nickname,
  token,
  onSaved,
}: {
  application: CoachApplication;
  nickname: string | null;
  token: string | null;
  onSaved: (a: CoachApplication) => void;
}) {
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function onPick(file: File | null) {
    if (!file || !acquire()) return;
    setError("");
    try {
      if (!file.type.startsWith("image/") && !/\.(jpe?g|png|webp|heic|heif)$/i.test(file.name)) {
        setError("写真ファイルを選んでください");
        return;
      }
      setStatus("アップロードしています…");
      const sending = await shrinkImage(file);
      if (sending.size > maxPhotoBytes) {
        setError("10MB以内の写真にしてください");
        return;
      }
      const signed = await apiFetch<CloudinarySignature>("/v1/uploads/image", { method: "POST", token });
      const uploaded = await uploadToCloudinary("image", sending, signed, () => {});
      if (uploaded.public_id !== signed.public_id) throw new Error("写真のアップロードに失敗しました");
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me/photo", {
        method: "PUT",
        token,
        body: JSON.stringify({ photo_public_id: uploaded.public_id }),
      });
      onSaved(d.application);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.photo_public_id || err.message : "写真を登録できませんでした");
    } finally {
      setStatus("");
      release();
    }
  }

  return (
    <div className="flex items-center gap-3">
      <CoachAvatar url={application.photo_url} name={nickname} size={64} />
      <div className="min-w-0 space-y-1 text-sm">
        <p className="font-semibold">顔写真</p>
        <label className="inline-flex cursor-pointer rounded-full bg-white/15 px-3 py-1.5 text-sm font-semibold text-slate-100">
          {application.photo_url ? "写真を変える" : "写真を選ぶ"}
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp,image/heic,image/heif"
            className="sr-only"
            disabled={pending}
            onChange={(e) => {
              void onPick(e.target.files?.[0] ?? null);
              e.target.value = "";
            }}
          />
        </label>
        <p className="text-xs text-slate-300">顔がはっきり写った写真にしてください。コーチ一覧に載ります。</p>
        {status ? <p className="text-xs text-slate-200">{status}</p> : null}
        {error ? <p className="text-xs text-red-400">{error}</p> : null}
      </div>
    </div>
  );
}
