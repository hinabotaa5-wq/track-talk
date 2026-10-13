"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { recruitmentSchema } from "@/lib/validators";
import { PREFECTURES } from "@/lib/prefectures";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import { UploadProgress } from "./upload-progress";
import type { Post } from "@/lib/types";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { useActionLock } from "@/lib/use-action-lock";
import { useAuth } from "./auth-provider";

const maxPhotoBytes = 10 * 1024 * 1024;
const maxPhotos = 3;
const fieldClass = "mt-1 placeholder:text-zinc-400";

function looksLikeImage(file: File) {
  return file.type.startsWith("image/") || /\.(jpe?g|png|webp|gif|heic|heif)$/i.test(file.name);
}

type KeptPhoto = { public_id: string; url: string };

type PhotoFile = { file: File; url: string };

export function RecruitmentForm({ initial, returnTo }: { initial?: Post; returnTo?: "me" }) {
  const { token, session } = useAuth();
  const router = useRouter();
  const recruitment = initial?.recruitment;
  const [title, setTitle] = useState(initial?.title ?? "");
  const [prefectures, setPrefectures] = useState<string[]>(recruitment?.prefectures ?? []);
  const [venue, setVenue] = useState(recruitment?.venue ?? "");
  const [schedule, setSchedule] = useState(recruitment?.schedule ?? "");
  const [genderRatio, setGenderRatio] = useState(recruitment?.gender_ratio ?? "");
  const [averageAge, setAverageAge] = useState(recruitment?.average_age ?? "");
  const [recruitAge, setRecruitAge] = useState(recruitment?.recruit_age ?? "");
  const [links, setLinks] = useState<string[]>(recruitment?.links?.length ? recruitment.links : [""]);
  const [body, setBody] = useState(initial?.body ?? "");
  const [kept, setKept] = useState<KeptPhoto[]>(recruitment?.photos ?? []);
  const [files, setFiles] = useState<PhotoFile[]>([]);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const { pending: submitting, acquire, release } = useActionLock();
  const filesRef = useRef(files);

  useEffect(() => {
    filesRef.current = files;
  }, [files]);

  useEffect(() => {
    return () => {
      for (const item of filesRef.current) URL.revokeObjectURL(item.url);
    };
  }, []);

  function togglePrefecture(code: string) {
    setPrefectures((current) =>
      current.includes(code) ? current.filter((item) => item !== code) : [...current, code],
    );
  }

  function addFiles(list: FileList | null) {
    const room = maxPhotos - kept.length - files.length;
    if (!list || room <= 0) return;
    const next = [...files];
    for (const file of Array.from(list).slice(0, room)) {
      next.push({ file, url: URL.createObjectURL(file) });
    }
    setFiles(next);
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!session) {
      router.push("/login");
      return;
    }
    setError("");
    setStatus("");
    setUpload(null);
    setUploadFile(null);
    const parsed = recruitmentSchema.safeParse({
      title,
      prefectures,
      venue,
      schedule,
      gender_ratio: genderRatio,
      average_age: averageAge,
      recruit_age: recruitAge,
      links: links.map((link) => link.trim()).filter(Boolean),
      body,
    });
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "入力を確認してください");
      return;
    }
    if (kept.length + files.length > maxPhotos) {
      setError("写真は3枚までです");
      return;
    }
    for (const item of files) {
      if (!looksLikeImage(item.file)) {
        setError("写真ファイルを選んでください");
        return;
      }
      if (item.file.size > maxPhotoBytes) {
        setError("10MB以内の写真にしてください");
        return;
      }
    }
    if (!acquire()) return;
    try {
      const imageIDs = kept.map((photo) => photo.public_id);
      for (const [index, item] of files.entries()) {
        setStatus(`写真を準備しています（${index + 1}/${files.length}）`);
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/image", { method: "POST", token });
        setStatus("");
        setUploadPhase("upload");
        setUploadFile(item.file);
        setUpload({ loaded: 0, total: item.file.size, percent: 0 });
        const result = await uploadToCloudinary("image", item.file, signed, setUpload);
        if (result.public_id !== signed.public_id) {
          throw new Error("写真のアップロードに失敗しました");
        }
        imageIDs.push(result.public_id);
      }
      setUpload((current) => (current ? { ...current, loaded: current.total, percent: 100 } : current));
      setUploadPhase("saving");
      const payload = { ...parsed.data, kind: "recruitment", image_public_ids: imageIDs };
      const post = initial
        ? await apiFetch<Post>(`/v1/posts/${initial.id}`, {
            method: "PATCH",
            token,
            body: JSON.stringify(payload),
          })
        : await apiFetch<Post>("/v1/posts", {
            method: "POST",
            token,
            body: JSON.stringify(payload),
          });
      router.push(returnTo === "me" ? `/posts/${post.id}?from=me` : `/posts/${post.id}`);
    } catch (e) {
      if (e instanceof ApiRequestError) {
        const field = e.fields ? Object.values(e.fields)[0] : "";
        setError(field || e.message);
      } else {
        setError(e instanceof Error ? e.message : "保存に失敗しました");
      }
      setStatus("");
      setUpload(null);
      setUploadFile(null);
      release();
    }
  }

  const photoCount = kept.length + files.length;

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <label className="block text-sm font-medium">
        チーム名
        <Input className={fieldClass} maxLength={120} value={title} onChange={(e) => setTitle(e.target.value)} />
      </label>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">活動場所</legend>
        <div className="max-h-48 overflow-y-auto rounded-md border border-zinc-300 p-2">
          <ul className="grid grid-cols-3 gap-1">
            {PREFECTURES.map((prefecture) => (
              <li key={prefecture.code}>
                <label className="flex min-h-11 items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={prefectures.includes(prefecture.code)}
                    onChange={() => togglePrefecture(prefecture.code)}
                  />
                  {prefecture.name}
                </label>
              </li>
            ))}
          </ul>
        </div>
        <Input
          className={fieldClass}
          maxLength={80}
          placeholder="〇〇競技場"
          value={venue}
          onChange={(e) => setVenue(e.target.value)}
          aria-label="活動場所の詳細"
        />
      </fieldset>
      <label className="block text-sm font-medium">
        活動時間
        <Input
          className={fieldClass}
          maxLength={80}
          placeholder="毎週火曜 19:00–21:00"
          value={schedule}
          onChange={(e) => setSchedule(e.target.value)}
        />
      </label>
      <label className="block text-sm font-medium">
        男女比
        <Input
          className={fieldClass}
          maxLength={80}
          placeholder="男性7：女性3"
          value={genderRatio}
          onChange={(e) => setGenderRatio(e.target.value)}
        />
      </label>
      <label className="block text-sm font-medium">
        平均年齢
        <Input
          className={fieldClass}
          maxLength={80}
          placeholder="25歳"
          value={averageAge}
          onChange={(e) => setAverageAge(e.target.value)}
        />
      </label>
      <label className="block text-sm font-medium">
        募集年齢
        <Input
          className={fieldClass}
          maxLength={80}
          placeholder="18〜30歳"
          value={recruitAge}
          onChange={(e) => setRecruitAge(e.target.value)}
        />
      </label>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">SNSのURL</legend>
        <p className="text-sm text-zinc-600">4件までです。</p>
        {links.map((link, index) => (
          <div key={index} className="flex items-center gap-2">
            <Input
              className="placeholder:text-zinc-400"
              type="url"
              inputMode="url"
              maxLength={500}
              placeholder="https://"
              value={link}
              aria-label={`SNSのURL ${index + 1}`}
              onChange={(e) =>
                setLinks((current) => current.map((item, i) => (i === index ? e.target.value : item)))
              }
            />
            {links.length > 1 ? (
              <button
                type="button"
                className="shrink-0 text-sm text-zinc-500"
                onClick={() => setLinks((current) => current.filter((_, i) => i !== index))}
              >
                削除
              </button>
            ) : null}
          </div>
        ))}
        <button
          type="button"
          className="text-sm text-blue-700 disabled:text-zinc-400"
          disabled={links.length >= 4}
          onClick={() => setLinks((current) => [...current, ""])}
        >
          URLを追加
        </button>
      </fieldset>
      <label className="block text-sm font-medium">
        詳細
        <Textarea rows={10} className="mt-1" maxLength={10000} value={body} onChange={(e) => setBody(e.target.value)} />
      </label>
      <div className="space-y-2">
        <p className="text-sm font-medium">チームの写真</p>
        <p className="text-sm text-zinc-600">3枚までです。なくても投稿できます。</p>
        {kept.length > 0 || files.length > 0 ? (
          <ul className="grid grid-cols-3 gap-2">
            {kept.map((photo) => (
              <li key={photo.public_id} className="space-y-1">
                <img src={photo.url} alt="" className="aspect-[4/3] w-full rounded-md bg-zinc-100 object-cover" />
                <button
                  type="button"
                  className="text-sm text-zinc-500"
                  onClick={() => setKept((current) => current.filter((item) => item.public_id !== photo.public_id))}
                >
                  削除
                </button>
              </li>
            ))}
            {files.map((preview) => (
              <li key={preview.url} className="space-y-1">
                <img src={preview.url} alt="" className="aspect-[4/3] w-full rounded-md bg-zinc-100 object-cover" />
                <button
                  type="button"
                  className="text-sm text-zinc-500"
                  onClick={() => {
                    URL.revokeObjectURL(preview.url);
                    setFiles((current) => current.filter((item) => item !== preview));
                  }}
                >
                  削除
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        {photoCount < maxPhotos ? (
          <input
            className="block w-full text-base"
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif,image/heic,image/heif"
            multiple
            onChange={(e) => {
              addFiles(e.target.files);
              e.target.value = "";
            }}
            disabled={submitting}
          />
        ) : null}
      </div>
      {upload ? (
        <UploadProgress
          noun="写真"
          phase={uploadPhase}
          progress={upload}
          fileName={uploadFile?.name}
          fileSize={uploadFile?.size}
        />
      ) : null}
      {status ? <p className="text-sm text-zinc-600">{status}</p> : null}
      {error ? <p className="text-sm text-red-600">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={submitting}>
        {submitting ? (initial ? "更新しています" : "送っています") : initial ? "更新する" : "投稿する"}
      </Button>
    </form>
  );
}
