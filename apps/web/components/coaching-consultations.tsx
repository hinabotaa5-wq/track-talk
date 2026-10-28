"use client";

import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, apiErrorText, ApiRequestError } from "@/lib/api";
import { uploadToCloudinary, type CloudinarySignature } from "@/lib/cloudinary-upload";
import { canClarify, consultationStatus, type Consultation, type ConsultationRoom } from "@/lib/coaching-consultations";
import { coachingMessagingEnabled, messagingClosedNote } from "@/lib/features";
import { formatDateTime, formatYen } from "@/lib/format";
import { maxReviewSeconds, tooLong, videoDuration } from "@/lib/review";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";

// 下書きはサーバーに保存してから購入へ進む。購入後も同じ部屋で再開できる。
export function CoachingConsultations({ rootID, kind, token, guardianToken }: { rootID: string; kind: "review" | "contract"; token: string | null; guardianToken?: string }) {
  const base = guardianToken ? `/v1/guardian/${encodeURIComponent(guardianToken)}` : "/v1/coaching";
  const roomPath = `${base}/${kind === "review" ? (guardianToken ? "reviews" : "requests") : "contracts"}/${rootID}/consultations`;
  const returnPath = kind === "review" ? `/coaching/requests/${rootID}` : `/coaching/personal/${rootID}`;
  const [data, setData] = useState<ConsultationRoom | null>(null);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    const d = await apiFetch<ConsultationRoom>(roomPath, { token });
    setData(d);
  }, [roomPath, token]);
  useEffect(() => {
    let live = true;
    const refresh = () => { apiFetch<ConsultationRoom>(roomPath, { token }).then((d) => { if (live) setData(d); }).catch((e) => { if (live) setError(apiErrorText(e, "追加相談を読み込めませんでした")); }); };
    refresh(); const timer = window.setInterval(refresh, 30000); window.addEventListener("focus", refresh);
    return () => { live = false; window.clearInterval(timer); window.removeEventListener("focus", refresh); };
  }, [roomPath, token]);
  if (!data) return error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null;
  const open = data.consultations.some((c) => ["draft", "awaiting_payment", "offered", "in_review"].includes(c.status));
  const draft = data.consultations.find((c) => c.status === "draft" && !c.purged);
  return <section className="space-y-4 rounded-2xl border border-white/20 bg-white/5 p-4" aria-label="追加相談">
    <h2 className="font-bold">追加相談</h2>
    <p className="text-sm text-slate-200">新しい質問1件に、コーチが1回答えます。回答への確認は7日以内に1回無料です。会話はこの部屋に残ります。</p>
    <p className="text-xs text-slate-300">コーチの受諾は48時間以内、受諾後の回答は72時間以内。辞退・未回答の期限切れはチケット返却または全額返金します。やりとりは終了後1年間保存します。</p>
    {data.consultations.filter((c) => c.status !== "draft").map((c) => <ConsultationCard key={c.id} consultation={c} role={data.role} base={base} token={token} guardian={!!guardianToken} reload={load} />)}
    {!coachingMessagingEnabled ? <p className="text-xs text-slate-300">{messagingClosedNote}</p> : null}
    {coachingMessagingEnabled && data.role === "requester" && (draft || data.can_purchase && !open) ? <ConsultationDraft key={draft?.id ?? "new"} draft={draft} roomPath={roomPath} token={token} funding={data.funding} price={draft?.price_yen ?? data.price_yen} requiresVideo={data.requires_video} returnPath={returnPath} reload={load} /> : null}
    {coachingMessagingEnabled && !data.can_purchase && data.role === "requester" && !draft ? <p className="text-xs text-slate-300">{data.block_reason}</p> : null}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
  </section>;
}

async function uploadVideo(file: File, token: string | null): Promise<string> {
  if (file.size > 100 * 1024 * 1024) throw new Error("動画は100MB以内にしてください");
  if (tooLong(await videoDuration(file))) throw new Error(`動画は${maxReviewSeconds}秒以内にしてください`);
  const signed = await apiFetch<CloudinarySignature>("/v1/uploads/coaching-video", { method: "POST", token });
  const uploaded = await uploadToCloudinary("video", file, signed, () => {});
  if (uploaded.public_id !== signed.public_id) throw new Error("アップロードに失敗しました");
  return uploaded.public_id;
}
function ConsultationDraft({ draft, roomPath, token, funding, price, requiresVideo, returnPath, reload }: {
  draft?: Consultation; roomPath: string; token: string | null; funding: "ticket" | "cash"; price: number; requiresVideo: boolean; returnPath: string; reload: () => Promise<void>;
}) {
  const [body, setBody] = useState(draft?.question ?? "");
  const [file, setFile] = useState<File | null>(null);
  const [videoID, setVideoID] = useState(draft?.video_public_id ?? "");
  const [error, setError] = useState("");
  const [buy, setBuy] = useState(false);
  const requestKey = useRef(draft?.request_key ?? crypto.randomUUID());
  const { pending, acquire, release } = useActionLock();
  async function submit() {
    if (!acquire()) return;
    setError(""); setBuy(false);
    try {
      let video = videoID;
      if (file) { video = await uploadVideo(file, token); setVideoID(video); setFile(null); }
      const saved = await apiFetch<{ consultation: Consultation }>(roomPath, { method: "POST", token, body: JSON.stringify({ request_key: requestKey.current, body, video_public_id: video }) });
      const result = await apiFetch<{ checkout_url?: string }>(`/v1/coaching/consultations/${saved.consultation.id}/submit`, { method: "POST", token });
      if (result.checkout_url) { window.location.assign(result.checkout_url); return; }
      await reload();
    } catch (e) {
      setError(apiErrorText(e, e instanceof Error ? e.message : "送信できませんでした"));
      if (e instanceof ApiRequestError && e.code === "insufficient_tickets") setBuy(true);
    } finally { release(); }
  }
  return <div className="space-y-3">
    <p className="text-sm font-semibold">{funding === "ticket" ? `チケット1枚（${formatYen(price)}相当）` : formatYen(price)} / 質問1件と回答1件</p>
    <Textarea aria-label="追加相談の質問" value={body} onChange={(e) => setBody(e.target.value)} maxLength={2000} disabled={pending} placeholder="新しく相談したいこと" />
    <label className="block text-xs text-slate-200">動画（30秒・100MB以内{requiresVideo ? "、必須" : "、任意"}）<input type="file" accept="video/*" disabled={pending} onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="mt-2 block w-full text-sm" /></label>
    {videoID && !file ? <p className="text-xs text-slate-300">保存済みの動画を添付します。<button type="button" disabled={pending} onClick={() => setVideoID("")} className="underline">外す</button></p> : null}
    <p className="text-xs text-slate-300">購入前に下書きを7日間保存します。送信後は内容を変更できません。未成年の個別決済は保護者が支払います。</p>
    <Button type="button" disabled={pending || !body.trim() && !file && !videoID} onClick={() => void submit()}>{pending ? "保存・送信中…" : funding === "ticket" ? "質問する（チケット1枚）" : `質問する（${formatYen(price)}）`}</Button>
    {buy ? <Link className="block text-sm text-blue-300 underline" href={`/me/coaching/tickets?return_to=${encodeURIComponent(returnPath)}`}>チケットを購入して、この部屋へ戻る</Link> : null}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
  </div>;
}
function ConsultationCard({ consultation: c, role, base, token, guardian, reload }: { consultation: Consultation; role: ConsultationRoom["role"]; base: string; token: string | null; guardian: boolean; reload: () => Promise<void> }) {
  const [body, setBody] = useState(""); const [file, setFile] = useState<File | null>(null); const [error, setError] = useState(""); const [reported, setReported] = useState(false);
  const { pending, acquire, release } = useActionLock(); const confirmDialog = useConfirm();
  async function act(action: string) {
    if (!acquire()) return; setError("");
    try {
      const video = action === "answer" && file ? await uploadVideo(file, token) : "";
      const result = await apiFetch<{ checkout_url?: string }>(`${base}/consultations/${c.id}/${action}`, { method: "POST", token, body: JSON.stringify({ body, video_public_id: video }) });
      if (result.checkout_url) { window.location.assign(result.checkout_url); return; }
      setBody(""); setFile(null); await reload();
    } catch (e) { setError(apiErrorText(e, "送信できませんでした")); } finally { release(); }
  }
  async function report() {
    if (!(await confirmDialog(reportConfirmOptions({ title: "この追加相談を運営に通報しますか？", body: "通報すると、運営がこの相談の内容を確認できます。" }))) || !acquire()) return;
    try {
      await apiFetch(guardian ? `${base}/consultations/${c.id}/report` : "/v1/reports", { method: "POST", token, body: JSON.stringify({ target_type: "coaching_consultation", target_id: c.id, reason: "other", detail: "" }) });
      setReported(true);
    } catch (e) { if (e instanceof ApiRequestError && e.status === 409) setReported(true); else setError(apiErrorText(e, "通報できませんでした")); } finally { release(); }
  }
  const reply = role === "coach" && c.status === "in_review";
  const clarify = role === "requester" && canClarify(c);
  const clarifyReply = role === "coach" && c.status === "answered" && !!c.clarify_body && !c.clarification;
  return <article className="space-y-3 border-t border-white/20 pt-3">
    <p className="text-sm font-semibold">{consultationStatus[c.status]} · {c.funding === "ticket" ? "チケット1枚" : formatYen(c.price_yen)}</p>
    {c.hidden || c.purged ? <p className="text-sm text-slate-300">{c.hidden ? "運営が確認のため非表示にしています" : "保存期間が終了しました"}</p> : <>
      <p className="whitespace-pre-wrap text-sm">質問：{c.question}</p>
      {c.video_url ? <video src={c.video_url} controls playsInline preload="metadata" className="w-full rounded-lg" /> : null}
      {c.answer ? <p className="whitespace-pre-wrap text-sm">コーチ：{c.answer}</p> : null}
      {c.answer_video_url ? <video src={c.answer_video_url} controls playsInline preload="metadata" className="w-full rounded-lg" /> : null}
      {c.clarify_body ? <p className="whitespace-pre-wrap text-sm">回答への確認：{c.clarify_body}</p> : null}
      {c.clarification ? <p className="whitespace-pre-wrap text-sm">補足：{c.clarification}</p> : null}
      {c.status === "offered" && c.respond_by ? <p className="text-xs text-slate-300">受諾期限：{formatDateTime(c.respond_by)}</p> : null}
      {c.status === "in_review" && c.answer_by ? <p className="text-xs text-slate-300">回答期限：{formatDateTime(c.answer_by)}</p> : null}
      {role === "coach" && c.status === "offered" ? <div className="flex gap-3"><Button disabled={pending} onClick={() => void act("accept")}>引き受ける</Button><Button disabled={pending} onClick={() => void act("decline")}>辞退する</Button></div> : null}
      {coachingMessagingEnabled && (reply || clarify || clarifyReply) ? <div className="space-y-2">
        <Textarea aria-label={reply ? "コーチの回答" : clarify ? "回答への確認質問" : "コーチの補足"} maxLength={2000} value={body} onChange={(e) => setBody(e.target.value)} disabled={pending} />
        {reply ? <label className="block text-xs">実践動画（30秒・100MB以内、任意）<input type="file" accept="video/*" disabled={pending} onChange={(e) => setFile(e.target.files?.[0] ?? null)} /></label> : null}
        <Button disabled={pending || !body.trim() && !file} onClick={() => void act(reply ? "answer" : clarify ? "clarify" : "clarification")}>{reply ? "回答を送る" : clarify ? "回答について確認する（1回無料）" : "補足を送る"}</Button>
      </div> : null}
      {c.status === "awaiting_payment" && (role === "guardian" || role === "requester" && !c.minor) ? <Button disabled={pending} onClick={() => void act("checkout")}>お支払いへ</Button> : null}
      {c.status === "awaiting_payment" && c.minor && role === "requester" ? <p className="text-sm">保護者に、この契約のページからお支払いをお願いしてください。保護者リンクはマイコーチングで確認できます。</p> : null}
      {c.status === "awaiting_payment" && role === "requester" ? <Button disabled={pending} onClick={() => void act("cancel")}>支払い前の質問を取り下げる</Button> : null}
      {((role === "coach" || role === "guardian") && !["draft", "awaiting_payment"].includes(c.status) || role === "requester" && c.answered_at) ? <button disabled={pending || reported} onClick={() => void report()} className="text-xs underline">{reported ? "通報しました" : "この追加相談を通報する"}</button> : null}
    </>}
    {c.payment_state === "refund_pending" || c.payment_state === "refunded" ? <p className="text-sm text-slate-300">{c.payment_state === "refunded" ? "全額返金済み" : "全額返金を処理中"}</p> : null}
    {c.funding === "ticket" && ["declined", "expired", "cancelled"].includes(c.status) ? <p className="text-xs text-slate-300">確保したチケットは残高に戻ります。</p> : null}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
  </article>;
}

export function CoachConsultationQueue({ token }: { token: string | null }) {
  const [rows, setRows] = useState<{ id: string; review_request_id: string | null; contract_id: string | null; nickname: string | null; status: Consultation["status"] }[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    const refresh = () => { apiFetch<{ consultations: typeof rows }>("/v1/coach/consultations", { token }).then((d) => { if (live) setRows(d.consultations); }).catch(() => { if (live) setError("追加相談を読み込めませんでした"); }); };
    refresh(); const t = window.setInterval(refresh, 30000); return () => { live = false; window.clearInterval(t); };
  }, [token]);
  return <section className="space-y-2"><h2 className="font-bold">対応待ちの追加相談</h2>{rows.length ? rows.map((r) => <Link key={r.id} href={r.contract_id ? `/coaching/personal/${r.contract_id}` : `/coach/requests/${r.review_request_id}`} className="block text-sm text-blue-300 underline">{r.nickname ?? "利用者"}さん · {consultationStatus[r.status]}</Link>) : <p className="text-sm text-slate-300">現在はありません</p>}{error ? <p className="text-sm text-red-400">{error}</p> : null}</section>;
}
