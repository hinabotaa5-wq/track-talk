"use client";

import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError, apiErrorText } from "@/lib/api";
import { ComposerPage } from "@/components/composer-page";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  staleUploadMessage,
  uploadToCloudinary,
  type CloudinarySignature,
  type CloudinaryUploadProgress,
} from "@/lib/cloudinary-upload";
import type { Post } from "@/lib/types";
import { clearHomeFeeds } from "@/lib/home-feed-cache";
import { useActionLock } from "@/lib/use-action-lock";

const maxBytes = 100 * 1024 * 1024;

// Cloudinary は発行から1時間を過ぎた署名を断る。余裕を見て、50分を過ぎた先読みの署名は使わずに取り直す。
const signatureMaxAgeMs = 50 * 60 * 1000;

type SignatureEntry = { signed: CloudinarySignature; at: number };
type UploadState = "idle" | "uploading" | "done" | "failed";

function isAbort(err: unknown) {
  return err instanceof DOMException && err.name === "AbortError";
}

function formatSize(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

// 待ち時間を「消す」のではなく「重ねる」画面。
// - 署名は、ファイル選択ボタンを押した時点で取りにいく（写真ピッカーを開いている間に並行して終わる）。
//   画面を開いただけで取ると、投稿しない訪問でも1日30回の署名の上限に数えられてしまうため、タップ時にしている。
//   使わなかった先読みの署名はこの画面にいる間は取っておき、次の選択で使い回す（上限を余計に使わない）。
// - 動画を選んだらすぐに送り始め、送っている間も詳細は書ける。投稿ボタンは保存（POST /v1/posts）だけにする。
export default function OwnVideoPage() {
  const { token } = useAuth();
  const router = useRouter();
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const uploaded = useRef<{ file: File; id: string } | null>(null);
  const [error, setError] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadState, setUploadState] = useState<UploadState>("idle");
  const { pending: submitting, acquire, release } = useActionLock();

  // 先読みした署名（取得中の Promise ごと持つ）。使ったら空にする。
  const spare = useRef<Promise<SignatureEntry> | null>(null);
  // 進行中のアップロード。選び直したら止める。
  const inflight = useRef<AbortController | null>(null);
  // 選び直したあとに古いアップロードの結果が届いても、画面に反映しないための番号。
  const uploadSeq = useRef(0);
  const previewRef = useRef<string | null>(null);

  useEffect(() => {
    return () => {
      inflight.current?.abort();
      if (previewRef.current) URL.revokeObjectURL(previewRef.current);
    };
  }, []);

  function fetchSignature(): Promise<SignatureEntry> {
    return apiFetch<CloudinarySignature>("/v1/uploads/video", { method: "POST", token }).then((signed) => ({
      signed,
      at: Date.now(),
    }));
  }

  function prefetchSignature() {
    if (spare.current || !token) return;
    const pending = fetchSignature();
    spare.current = pending;
    // 先読みの失敗は見せない。送り始めるときに取り直し、そこで失敗したら伝える。
    pending.catch(() => {
      if (spare.current === pending) spare.current = null;
    });
  }

  async function takeSignature(): Promise<SignatureEntry & { prefetched: boolean }> {
    const pending = spare.current;
    spare.current = null;
    if (pending) {
      try {
        const entry = await pending;
        if (Date.now() - entry.at < signatureMaxAgeMs) return { ...entry, prefetched: true };
      } catch {
        // 取り直す
      }
    }
    return { ...(await fetchSignature()), prefetched: false };
  }

  async function startUpload(target: File) {
    inflight.current?.abort();
    const controller = new AbortController();
    inflight.current = controller;
    const seq = ++uploadSeq.current;
    setError("");

    if (uploaded.current?.file === target) {
      // 同じ動画を選び直したときや、保存だけ失敗したあとは、上げ済みの動画を使い回す（孤児を増やさない）。
      setUpload({ loaded: target.size, total: target.size, percent: 100 });
      setUploadState("done");
      return;
    }
    if (target.size > maxBytes) {
      setUpload(null);
      setUploadState("failed");
      setError("100MB以内の動画にしてください");
      return;
    }

    setUploadState("uploading");
    setUpload({ loaded: 0, total: target.size, percent: 0 });
    const onProgress = (progress: CloudinaryUploadProgress) => {
      if (seq === uploadSeq.current) setUpload(progress);
    };
    try {
      let entry = await takeSignature();
      if (controller.signal.aborted) {
        // 選び直しで使わなくなった署名は、次のアップロード用に取っておく。
        if (!spare.current) spare.current = Promise.resolve(entry);
        return;
      }
      let payload;
      try {
        payload = await uploadToCloudinary("video", target, entry.signed, onProgress, controller.signal);
      } catch (err) {
        // 先読みした署名が期限切れで断られたときは、取り直して1回だけやり直す（失敗は見せない）。
        if (!(entry.prefetched && err instanceof Error && err.message === staleUploadMessage)) throw err;
        entry = { ...(await fetchSignature()), prefetched: false };
        if (controller.signal.aborted) return;
        onProgress({ loaded: 0, total: target.size, percent: 0 });
        payload = await uploadToCloudinary("video", target, entry.signed, onProgress, controller.signal);
      }
      if (payload.public_id !== entry.signed.public_id) {
        throw new Error("アップロードに失敗しました");
      }
      if (seq !== uploadSeq.current) return;
      uploaded.current = { file: target, id: payload.public_id };
      setUpload({ loaded: target.size, total: target.size, percent: 100 });
      setUploadState("done");
    } catch (err) {
      if (isAbort(err) || seq !== uploadSeq.current) return;
      setUpload(null);
      setUploadState("failed");
      setError(apiErrorText(err, err instanceof Error ? err.message : "アップロードに失敗しました"));
    } finally {
      if (inflight.current === controller) inflight.current = null;
    }
  }

  function onFileChange(e: ChangeEvent<HTMLInputElement>) {
    const next = e.target.files?.[0] ?? null;
    if (previewRef.current) URL.revokeObjectURL(previewRef.current);
    previewRef.current = next ? URL.createObjectURL(next) : null;
    setPreview(previewRef.current);
    setFile(next);
    if (next) {
      void startUpload(next);
      return;
    }
    inflight.current?.abort();
    uploadSeq.current++;
    setUpload(null);
    setUploadState("idle");
    setError("");
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!file) {
      setError("動画ファイルを選んでください");
      return;
    }
    // 送っている間はボタンを押せない（二重送信にしない）。送り終わるまで待つ。
    if (uploadState === "uploading") return;
    if (uploaded.current?.file !== file) {
      // 送るのに失敗していたら、ボタンで送り直す。
      void startUpload(file);
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch<Post>("/v1/posts", {
        method: "POST",
        token,
        body: JSON.stringify({
          body,
          kind: "video",
          video_public_id: uploaded.current.id,
        }),
      });
      clearHomeFeeds();
      router.push("/?sort=new");
    } catch (err) {
      // 動画そのものを断られたとき（項目の指定がないエラー）は、上げ直しが必要なので使い回さない。
      if (!(err instanceof ApiRequestError && err.fields)) {
        uploaded.current = null;
        setUpload(null);
        setUploadState("failed");
      }
      setError(apiErrorText(err, err instanceof Error ? err.message : "保存に失敗しました"));
      release();
    }
  }

  const percent = upload ? Math.min(uploadState === "done" ? 100 : 99, Math.round(upload.percent)) : 0;
  const buttonLabel = !file
    ? "動画を選んでください"
    : uploadState === "uploading"
      ? `動画を送っています ${percent}%`
      : uploadState === "failed"
        ? "動画を送り直す"
        : submitting
          ? "投稿しています"
          : "投稿する";

  return (
    <ComposerPage title="自分の動画">
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium">
            動画ファイル
            <input
              className="mt-1 block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="video/mp4,video/quicktime,video/webm"
              onClick={prefetchSignature}
              onChange={onFileChange}
              disabled={submitting}
              required
            />
          </label>
          {/*
            iPhone の写真ピッカーは、✓を押してから動画を書き出し終えるまで（17秒の動画で約15秒）ページにファイルを渡さない。
            この待ちは Web 側では縮められないので、このひとことは時間を短くするためではなく、
            固まったと思って離れたり連打したりしないように伝えるためのもの。
          */}
          <p className="mt-1 text-xs text-slate-400">
            動画を選んで✓を押すと、iPhoneの準備で少し時間がかかることがあります。そのままお待ちください
          </p>
        </div>
        {file ? (
          <div className="flex items-center gap-3 rounded-2xl border border-white/10 bg-white/5 p-2" data-testid="video-upload-status">
            {preview ? (
              <video
                src={`${preview}#t=0.1`}
                className="h-16 w-16 shrink-0 rounded-xl bg-black object-cover"
                muted
                playsInline
                preload="metadata"
                aria-label="選んだ動画のプレビュー"
              />
            ) : null}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm text-slate-200">{file.name}</p>
              <p className="text-xs tabular-nums text-slate-400">
                {formatSize(file.size)}
                {uploadState === "uploading" ? ` ・ 送っています ${percent}%` : null}
                {uploadState === "done" ? " ・ 送信済み" : null}
                {uploadState === "failed" ? " ・ 送れませんでした" : null}
              </p>
              {uploadState === "uploading" || uploadState === "done" ? (
                <div
                  className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/12"
                  role="progressbar"
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-valuenow={percent}
                  aria-valuetext={`${percent}パーセント`}
                  aria-label="動画の送信"
                >
                  <div
                    className="h-full rounded-full bg-blue-600 transition-[width] duration-150 ease-out"
                    style={{ width: `${percent}%` }}
                  />
                </div>
              ) : null}
            </div>
          </div>
        ) : null}
        <label className="block text-sm font-medium">
          詳細
          <Textarea rows={3} className="mt-1" value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button
          type="submit"
          className="w-full tabular-nums"
          disabled={submitting || !file || uploadState === "uploading"}
        >
          {buttonLabel}
        </Button>
      </form>
    </ComposerPage>
  );
}
