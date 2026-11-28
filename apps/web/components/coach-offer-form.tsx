"use client";
import { L } from "@/lib/i18n";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { eagerVideoQuery, uploadVideoToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import {
  coachOfferItemsError,
  fixedOfferTitles,
  maxCoachDemoVideos,
  maxCoachOfferItemChars,
  maxCoachOfferItems,
  type CoachDemoVideo,
  type CoachOffers,
} from "@/lib/coach-offer";
import { formatDateTime } from "@/lib/format";
import { maxReviewSeconds, tooLong, videoDuration } from "@/lib/review";
import { useActionLock } from "@/lib/use-action-lock";
import { useEffect, useState, type FormEvent } from "react";

// 添削項目（追加項目）と見本動画の申請（本体 #1086）。価格の申請画面に同居させ、価格とは別々に出せる。
// 申請は「追加項目の希望セット（全件）＋見本動画の希望セット（全件）」の1件。承認待ちのあいだは上書き・取り下げができる。

// 見本動画の容量の上限は、回答の見本動画（コーチの回答フォーム）と同じ。長さは申込動画と同じ上限（maxReviewSeconds）。
const maxDemoBytes = 100 * 1024 * 1024;

// 希望セットの見本動画1本。登録済み（public_id あり）か、これから上げるファイル。
type DraftVideo = { key: string; existing?: CoachDemoVideo; file?: File };

function initialDraft(data: CoachOffers): { items: string[]; videos: DraftVideo[] } {
  const base = data.pending ?? data.approved ?? { items: data.items, demo_videos: data.demo_videos };
  return {
    items: [...base.items],
    videos: base.demo_videos.filter((v) => v.public_id).map((v, i) => ({ key: `e${i}-${v.public_id}`, existing: v })),
  };
}

export function CoachOfferSection({ token }: { token: string | null | undefined }) {
  // 本体 #1108：申請内容は「どの利用者の分か」（ここでは token）と組で持ち、表示中の利用者と一致するときだけ出す（#1011 の方式に揃える）。
  const [loaded, setLoaded] = useState<{ key: string | null; data: CoachOffers | null }>({ key: null, data: null });
  const [loadErrorLoaded, setLoadErrorLoaded] = useState<{ key: string | null; message: string }>({ key: null, message: "" });
  const data = token && loaded.key === token ? loaded.data : null;
  const loadError = token && loadErrorLoaded.key === token ? loadErrorLoaded.message : "";
  // 申請・取り下げのあとは編集欄を作り直すので、結果の知らせはここで持つ（知らせも利用者と組で持つ）。
  const [messageLoaded, setMessageLoaded] = useState<{ key: string | null; message: string }>({ key: null, message: "" });
  const message = token && messageLoaded.key === token ? messageLoaded.message : "";

  useEffect(() => {
    if (!token) return;
    let alive = true;
    const load = () => {
      apiFetch<CoachOffers>("/v1/coach/me/offers", { token })
        .then((d) => { if (alive) { setLoaded({ key: token, data: d }); setLoadErrorLoaded({ key: token, message: "" }); } })
        .catch(() => { if (alive) setLoadErrorLoaded({ key: token, message: L("添削項目・見本動画を読み込めませんでした") }); });
    };
    const refresh = () => { if (document.visibilityState === "visible") load(); };
    // 履歴キャッシュ（bfcache）から戻ったときも取り直す（本体 #1011 と同じ契機）。
    const restored = (event: PageTransitionEvent) => { if (event.persisted) load(); };
    load();
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("focus", refresh);
    window.addEventListener("pageshow", restored);
    return () => {
      alive = false;
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("pageshow", restored);
    };
  }, [token]);

  return (
    <section aria-label={L("添削項目・見本動画")} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
      <h2 className="font-semibold text-white">{L("添削項目・見本動画")}</h2>
      <p className="text-sm leading-relaxed text-slate-200">
        {L("すべての回答に入る4項目に加えて、あなたが添削する項目を5件まで追加できます。見本動画はコーチ詳細に3本まで載せられます。運営が承認したあと、承認の時点で対応中だった依頼がすべて終わってから切り替わります。進行中の依頼は申し込まれたときの項目のままです。")}
      </p>
      {loadError ? <p role="alert" className="text-sm text-red-400">{loadError}</p> : null}
      {data ? <OfferEditor key={`${data.pending?.updated_at ?? ""}-${data.approved?.id ?? ""}-${data.items.join("|")}`} data={data} token={token} onChange={(d, note) => { setLoaded({ key: token ?? null, data: d }); setMessageLoaded({ key: token ?? null, message: note }); }} message={message} onEdit={() => setMessageLoaded({ key: token ?? null, message: "" })} /> : null}
    </section>
  );
}

function OfferSetView({ label, items, videos, testID }: { label: string; items: readonly string[]; videos: readonly CoachDemoVideo[]; testID: string }) {
  return (
    <div className="space-y-1 text-sm" data-testid={testID}>
      <p className="text-slate-300">{label}</p>
      <ul className="list-disc space-y-0.5 pl-5 break-words">
        {items.length === 0 ? <li className="text-slate-300">{L("追加項目なし")}</li> : items.map((item) => <li key={item}>{item}</li>)}
      </ul>
      <p className="text-xs text-slate-300">{L("見本動画 {n}本", { n: videos.length })}</p>
    </div>
  );
}

function OfferEditor({ data, token, onChange, message, onEdit }: { data: CoachOffers; token: string | null | undefined; onChange: (d: CoachOffers, message: string) => void; message: string; onEdit: () => void }) {
  const initial = initialDraft(data);
  const [items, setItems] = useState<string[]>(initial.items);
  const [videos, setVideos] = useState<DraftVideo[]>(initial.videos);
  const [error, setError] = useState("");
  const [upload, setUpload] = useState<{ file: File; progress: CloudinaryUploadProgress } | null>(null);
  const { pending, acquire, release } = useActionLock();
  const maxItems = data.max_items || maxCoachOfferItems;
  const maxVideos = data.max_demo_videos || maxCoachDemoVideos;

  async function pickVideo(file: File | null) {
    setError("");
    if (!file) return;
    // 種別（MIME）が動画でないファイルは、容量・長さを見る前に知らせる（本体 #1109）。
    if (!file.type.startsWith("video/")) { setError(L("動画ファイルを選んでください")); return; }
    if (file.size > maxDemoBytes) { setError(L("見本動画のサイズが大きすぎます（100MBまで）")); return; }
    if (tooLong(await videoDuration(file))) { setError(L("見本動画は{seconds}秒以内にしてください", { seconds: maxReviewSeconds })); return; }
    setVideos((prev) => (prev.length >= maxVideos ? prev : [...prev, { key: `f${Date.now()}-${file.name}`, file }]));
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    onEdit();
    const titles = items.map((v) => v.trim());
    const itemsError = coachOfferItemsError(titles);
    if (itemsError) { setError(itemsError); return; }
    if (!acquire()) return;
    try {
      const ids: string[] = [];
      const uploaded: DraftVideo[] = [];
      for (const v of videos) {
        if (v.existing?.public_id) { ids.push(v.existing.public_id); uploaded.push(v); continue; }
        if (!v.file) continue;
        const signed = await apiFetch<CloudinarySignature>(`/v1/uploads/coaching-video${eagerVideoQuery}`, { method: "POST", token });
        setUpload({ file: v.file, progress: { loaded: 0, total: v.file.size, percent: 0 } });
        const file = v.file;
        const payload = await uploadVideoToCloudinary(file, signed, token ?? null, (progress) => setUpload({ file, progress }));
        if (payload.public_id !== signed.public_id) throw new Error(L("アップロードに失敗しました"));
        ids.push(payload.public_id);
        // 送り直すときに同じ動画を上げ直さない。
        uploaded.push({ key: v.key, existing: { url: "", public_id: payload.public_id } });
      }
      setVideos(uploaded);
      const d = await apiFetch<CoachOffers>("/v1/coach/me/offers", { method: "PUT", token, body: JSON.stringify({ items: titles, demo_video_public_ids: ids }) });
      onChange(d, data.pending ? L("申請の内容を変更しました") : L("添削項目・見本動画を申請しました。運営の承認をお待ちください。"));
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.items || err.fields?.demo_video_public_ids || err.message : err instanceof Error ? err.message : L("申請できませんでした"));
    } finally {
      setUpload(null);
      release();
    }
  }

  async function withdraw() {
    if (!acquire()) return;
    setError("");
    onEdit();
    try {
      onChange(await apiFetch<CoachOffers>("/v1/coach/me/offers/withdraw", { method: "POST", token }), L("申請を取り下げました"));
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("取り下げられませんでした"));
    } finally {
      release();
    }
  }

  return (
    <>
      <OfferSetView label={L("いまの追加項目")} items={data.items} videos={data.demo_videos} testID="offer-current" />
      {data.approved ? (
        <div className="space-y-1">
          <OfferSetView label={L("切り替え待ち")} items={data.approved.items} videos={data.approved.demo_videos} testID="offer-approved" />
          {data.approved.approved_at ? (
            <p className="text-xs text-slate-300">
              {L("{v0}に承認されました。承認の時点で対応中だった依頼がすべて終わると切り替わります。", { v0: formatDateTime(data.approved.approved_at) })}
            </p>
          ) : null}
        </div>
      ) : null}
      {data.pending ? <OfferSetView label={L("承認待ち")} items={data.pending.items} videos={data.pending.demo_videos} testID="offer-pending" /> : null}
      <form onSubmit={(e) => void submit(e)} className="space-y-3">
        <div className="space-y-1 text-sm">
          <p className="font-semibold">{L("すべての回答に入る項目（変更できません）")}</p>
          <ul className="list-disc space-y-0.5 pl-5 text-slate-200">
            {fixedOfferTitles().map((title) => <li key={title}>{title}</li>)}
          </ul>
        </div>
        <fieldset className="space-y-2 text-sm">
          <legend className="font-semibold">{L("追加項目（{max}件まで・各{chars}文字まで）", { max: maxItems, chars: data.max_item_chars || maxCoachOfferItemChars })}</legend>
          {items.map((item, i) => (
            <div key={i} className="flex items-center gap-2">
              <Input
                aria-label={L("追加項目 {n}", { n: i + 1 })}
                value={item}
                placeholder={L("例：スタート局面の詳しい分析")}
                onChange={(e) => setItems((prev) => prev.map((v, j) => (j === i ? e.target.value : v)))}
                className="min-w-0 flex-1"
              />
              <Button type="button" className="shrink-0 bg-white/15 hover:bg-white/25" disabled={pending} onClick={() => setItems((prev) => prev.filter((_, j) => j !== i))}>
                {L("削除")}
              </Button>
            </div>
          ))}
          {items.length < maxItems ? (
            <Button type="button" className="w-full bg-white/15 hover:bg-white/25" disabled={pending} onClick={() => setItems((prev) => [...prev, ""])}>
              {L("追加項目を足す")}
            </Button>
          ) : null}
          {items.length === 0 ? <p className="text-xs text-slate-300">{L("例：スタート局面の詳しい分析")}</p> : null}
        </fieldset>
        <fieldset className="space-y-2 text-sm">
          <legend className="font-semibold">{L("見本動画（{max}本まで・{seconds}秒以内）", { max: maxVideos, seconds: maxReviewSeconds })}</legend>
          {videos.map((v, i) => (
            <div key={v.key} className="flex items-center gap-2">
              {v.existing?.url ? (
                <video src={v.existing.url} controls playsInline preload="metadata" className="w-32 max-h-[160px] shrink-0 rounded-lg bg-black" />
              ) : null}
              <span className="min-w-0 flex-1 break-words text-slate-200">{v.file ? v.file.name : L("見本動画 {n}", { n: i + 1 })}</span>
              <Button type="button" className="shrink-0 bg-white/15 hover:bg-white/25" disabled={pending} onClick={() => setVideos((prev) => prev.filter((x) => x.key !== v.key))}>
                {L("外す")}
              </Button>
            </div>
          ))}
          {videos.length < maxVideos ? (
            <input
              aria-label={L("見本動画を選ぶ")}
              className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="video/mp4,video/quicktime,video/webm"
              onChange={(e) => { void pickVideo(e.target.files?.[0] ?? null); e.target.value = ""; }}
              disabled={pending}
            />
          ) : null}
        </fieldset>
        {upload ? <UploadProgress noun="動画" phase="upload" progress={upload.progress} fileName={upload.file.name} fileSize={upload.file.size} /> : null}
        {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
        {message ? <p role="status" className="text-sm text-green-300">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          {data.pending ? L("申請の内容を変更する") : L("この内容で申請する")}
        </Button>
        {data.pending ? (
          <Button type="button" className="w-full bg-white/15 hover:bg-white/25" disabled={pending} onClick={() => void withdraw()}>
            {L("申請を取り下げる")}
          </Button>
        ) : null}
      </form>
    </>
  );
}
