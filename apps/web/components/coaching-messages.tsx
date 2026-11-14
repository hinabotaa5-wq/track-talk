"use client";
import { L } from "@/lib/i18n";

import { ContactNotice, useSolicitationGuard } from "@/components/contact-notice";
import { PageLoading } from "@/components/page-loading";

import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { UploadProgress } from "@/components/upload-progress";
import { apiDownloadText, apiFetch, ApiRequestError, apiErrorText } from "@/lib/api";
import { eagerVideoQuery, uploadVideoToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { coachingMessagingEnabled, messagingClosedNote } from "@/lib/features";
import { formatDateTime } from "@/lib/format";
import { maxReviewSeconds, tooLong, videoDuration } from "@/lib/review";
import type { CoachingSupport } from "@/lib/coaching-support";
import type { CoachingContract, CoachingMessage, PlanMenu } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";

// パーソナル指導のやりとり欄。期間中は文章と動画を送れ、コーチはメニューも作って送れる。
// 契約が終わった時点（満了・途中終了とも）で、選手・コーチ・保護者のだれからも見られなくなる。終わるまでのあいだに、
// 選手とコーチは文章とメニューをダウンロードできる。通報・紛争の対応中は、対応が終わるまで閉じずに読むだけにする。
// 動画ファイルは契約の終わりから30日で削除する（運営者決定 2026-11-10、本体 #506）。
// 18歳未満のときは保護者がリンクから読む（送信はできない）。相手の1通ごとに通報できる。
// リアルタイムではないので、開いている間は30秒ごとと、画面に戻ったときに読み直す。

const maxVideoBytes = 100 * 1024 * 1024;
const maxBodyChars = 2000;

function senderName(m: CoachingMessage, contract: CoachingContract): string {
  if (m.mine) return L("あなた");
  if (!m.sender_active) return L("退会したユーザー");
  if (m.sender_role === "coach") return contract.coach?.nickname ?? L("コーチ");
  if (m.sender_role === "guardian") return L("保護者");
  return L("{v0}さん", { v0: contract.requester?.nickname ?? L("名前なし") });
}

function MenuCard({ menu }: { menu: PlanMenu }) {
  return (
    <div className="space-y-1.5 rounded-lg border border-blue-400/40 bg-blue-500/10 p-2">
      <p className="text-xs font-semibold text-blue-300">{L("練習メニュー")}</p>
      <p className="font-semibold text-white">{menu.title}</p>
      {menu.period ? <p className="text-xs text-slate-300">{menu.period}</p> : null}
      {menu.note ? <p className="whitespace-pre-wrap text-sm text-slate-100">{menu.note}</p> : null}
      <dl className="space-y-1">
        {menu.days.map((d, i) => (
          <div key={i} className="grid grid-cols-[4.5rem_1fr] gap-2">
            <dt className="text-xs text-slate-300">{d.day}</dt>
            <dd className="whitespace-pre-wrap text-slate-100">{d.content}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

// やりとり欄の API の場所。保護者はリンクのトークンで読み書きする。
type Access = { token: string | null; guardianToken?: string };

function contractPath(contract: CoachingContract, access: Access): string {
  return access.guardianToken
    ? `/v1/guardian/${access.guardianToken}/contracts/${contract.id}`
    : `/v1/coaching/contracts/${contract.id}`;
}

function ReportButton({ message, access }: { message: CoachingMessage; access: Access }) {
  const [state, setState] = useState<"idle" | "done" | "error">("idle");
  const { pending, acquire, release } = useActionLock();
  const confirmDialog = useConfirm();

  async function report() {
    if (!(await confirmDialog(reportConfirmOptions({ title: L("このメッセージを運営に通報しますか？"), body: L("通報すると、運営がこのやりとりを確認します。") })))) return;
    if (!acquire()) return;
    try {
      if (access.guardianToken) {
        await apiFetch(`/v1/guardian/${access.guardianToken}/messages/${message.id}/report`, {
          method: "POST",
          body: JSON.stringify({ reason: "other", detail: "" }),
        });
      } else {
        await apiFetch("/v1/reports", {
          method: "POST",
          token: access.token,
          body: JSON.stringify({ target_type: "coaching_message", target_id: message.id, reason: "other", detail: "" }),
        });
      }
      setState("done");
    } catch (err) {
      setState(err instanceof ApiRequestError && err.status === 409 ? "done" : "error");
    } finally {
      release();
    }
  }

  if (state === "done") return <p className="text-xs text-slate-300">{L("通報しました。運営が確認します。")}</p>;
  return (
    <button type="button" onClick={() => void report()} disabled={pending} className="text-xs text-slate-300 underline">
      {state === "error" ? L("通報できませんでした。もう一度") : L("通報する")}
    </button>
  );
}

function MessageItem({ message, contract, access }: { message: CoachingMessage; contract: CoachingContract; access: Access }) {
  return (
    <li id={`message-${message.id}`} className={`flex ${message.mine ? "justify-end" : "justify-start"}`}>
      <div
        className={`max-w-[85%] space-y-2 rounded-2xl p-3 text-sm ${
          message.mine ? "bg-blue-600/60 text-pure-white" : "border border-white/20 bg-white/10 text-slate-100"
        }`}
      >
        <p className="text-xs text-slate-300">
          {senderName(message, contract)} · {formatDateTime(message.created_at)}
        </p>
        {message.review_submission_id ? <a href={`#message-${message.review_submission_id}`} className="block text-xs text-blue-200 underline">{L("この動画への回答")}</a> : null}
        {message.body ? <p className="whitespace-pre-wrap">{message.body}</p> : null}
        {message.video_purged ? <p className="text-xs text-slate-300">{L("動画は保存期間（契約の終わりから30日）を過ぎたため削除しました。")}</p> : null}
        {message.video_url ? (
          <video
            src={message.video_url}
            poster={message.thumbnail_url || undefined}
            controls
            playsInline
            preload="metadata"
            className="w-full rounded-lg bg-black"
          />
        ) : null}
        {message.menu ? <MenuCard menu={message.menu} /> : null}
        {message.mine ? null : <ReportButton message={message} access={access} />}
      </div>
    </li>
  );
}

const emptyMenu = (): PlanMenu => ({ title: "", days: [{ day: "", content: "" }] });

function Composer({
  contract,
  access,
  onSent,
}: {
  contract: CoachingContract;
  access: Access;
  onSent: (m: CoachingMessage) => void;
}) {
  const token = access.token;
  // 保護者はログインしないので、動画のアップロードはできない。
  const canVideo = !access.guardianToken;
  const [reviewID, setReviewID] = useState("");
  const [support, setSupport] = useState<CoachingSupport | null>(null);
  useEffect(() => { if (contract.fulfillment_version !== 1 || contract.role !== "coach") return; let live = true;
    const load = () => { apiFetch<CoachingSupport>(`${contractPath(contract, access)}/support`, { token:access.token }).then((d) => { if (live) setSupport(d); }).catch(() => {}); };
    load(); const t = window.setInterval(load,30000); return () => { live = false; window.clearInterval(t); };
  }, [contract, access]);
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [menu, setMenu] = useState<PlanMenu | null>(null);
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [error, setError] = useState("");
  const [fileKey, setFileKey] = useState(0);
  const { pending, acquire, release } = useActionLock();
  const isCoach = contract.role === "coach";
  const guard = useSolicitationGuard();

  // 長さを確かめ終えてから選んだことにする(確かめているあいだに送れないように)。
  // 続けて別の動画を選んだら、前の動画の確認の結果は使わない。
  const pickSeq = useRef(0);
  async function onPickFile(picked: File | null) {
    const seq = ++pickSeq.current;
    setError("");
    setFile(null);
    if (!picked) return;
    if (picked.size > maxVideoBytes) {
      setError(L("100MB以内の動画にしてください"));
      return;
    }
    const long = tooLong(await videoDuration(picked));
    if (seq !== pickSeq.current) return;
    if (long) {
      setError(L("動画は{maxReviewSeconds}秒以内にしてください", { maxReviewSeconds }));
      return;
    }
    setFile(picked);
  }

  function setDay(i: number, key: "day" | "content", value: string) {
    setMenu((m) => (m ? { ...m, days: m.days.map((d, j) => (j === i ? { ...d, [key]: value } : d)) } : m));
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!body.trim() && !file && !menu) return;
    if (!guard.allow([body, menu?.title, menu?.period, menu?.note, ...(menu?.days ?? []).flatMap((d) => [d.day, d.content])].filter(Boolean).join("\n"))) return;
    if (!acquire()) return;
    setError("");
    try {
      let videoPublicID = "";
      if (file) {
        const signed = await apiFetch<CloudinarySignature>(`/v1/uploads/coaching-video${eagerVideoQuery}`, { method: "POST", token });
        setUploadPhase("upload");
        setUpload({ loaded: 0, total: file.size, percent: 0 });
        const payload = await uploadVideoToCloudinary(file, signed, token, setUpload);
        if (payload.public_id !== signed.public_id) throw new Error(L("アップロードに失敗しました"));
        videoPublicID = payload.public_id;
        setUploadPhase("saving");
      }
      const d = await apiFetch<{ message: CoachingMessage }>(`${contractPath(contract, access)}/messages`, {
        method: "POST",
        token,
        body: JSON.stringify({ body, video_public_id: videoPublicID, menu, review_submission_id:reviewID || undefined }),
      });
      onSent(d.message);
      setSupport((prev) => prev ? { ...prev, videos:prev.videos.filter((v) => v.question_id !== reviewID) } : prev);
      setReviewID("");
      setBody("");
      setFile(null);
      setFileKey((k) => k + 1);
      setMenu(null);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : L("送信に失敗しました")));
    } finally {
      setUpload(null);
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-3">
      {support ? <label className="block text-sm">{L("回答する動画")}<select aria-label={L("回答する動画")} value={reviewID} onChange={(e) => setReviewID(e.target.value)} className="mt-1 block w-full rounded border border-white/25 bg-slate-900 p-2"><option value="">{L("通常のメッセージ（回数に含めない）")}</option>{support.videos.filter((v) => !v.answer_id).map((v) => <option key={v.question_id} value={v.question_id}>{L("{v0} の動画", { v0: formatDateTime(v.created_at) })}</option>)}</select></label> : null}
      <Textarea
        rows={3}
        value={body}
        maxLength={maxBodyChars}
        onChange={(e) => setBody(e.target.value)}
        placeholder={
          isCoach
            ? L("練習の様子への返事や、次にやることを書きます")
            : access.guardianToken
              ? L("コーチへの質問や、家での様子を書きます")
              : L("練習の様子や、聞きたいことを書きます")
        }
        aria-label={L("メッセージ")}
      />
      {menu ? (
        <div className="space-y-2 rounded-xl border border-blue-400/40 bg-blue-500/10 p-2">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold text-blue-300">{L("練習メニューを作る（見直すときは新しく作って送ると、前のメニューも残ります）")}</p>
            <button type="button" className="text-xs text-slate-300 underline" onClick={() => setMenu(null)}>{L("メニューをやめる")}</button>
          </div>
          <Input
            value={menu.title}
            maxLength={60}
            onChange={(e) => setMenu({ ...menu, title: e.target.value })}
            placeholder={L("例: 1週目 スタートの練習")}
            aria-label={L("メニューの名前")}
          />
          <Input
            value={menu.period ?? ""}
            maxLength={40}
            onChange={(e) => setMenu({ ...menu, period: e.target.value })}
            placeholder={L("対象期間（例: 12/1〜12/7）")}
            aria-label={L("対象期間")}
          />
          <Textarea
            rows={2}
            value={menu.note ?? ""}
            maxLength={300}
            onChange={(e) => setMenu({ ...menu, note: e.target.value })}
            placeholder={L("全体への一言（例: 今週はスタートを固めます）")}
            aria-label={L("全体への一言")}
          />
          {menu.days.map((d, i) => (
            <div key={i} className="grid grid-cols-[5rem_1fr] gap-2">
              <Input value={d.day} maxLength={20} onChange={(e) => setDay(i, "day", e.target.value)} placeholder={L("月")} aria-label={L("{v0}日目の日付や曜日", { v0: i + 1 })} />
              <Textarea
                rows={2}
                value={d.content}
                maxLength={300}
                onChange={(e) => setDay(i, "content", e.target.value)}
                placeholder={L("例: 流し 100m×5、スタート 30m×6")}
                aria-label={L("{v0}日目の中身", { v0: i + 1 })}
              />
            </div>
          ))}
          {menu.days.length < 14 ? (
            <button
              type="button"
              className="text-xs text-blue-300 underline"
              onClick={() => setMenu({ ...menu, days: [...menu.days, { day: "", content: "" }] })}
            >{L("1日足す")}</button>
          ) : null}
        </div>
      ) : null}
      <div className="flex flex-wrap items-center gap-3 text-sm">
        {canVideo ? (
          <label className="cursor-pointer text-blue-300 underline">
            {file ? L("動画: {name}", { name: file.name }) : L("動画をつける（{maxReviewSeconds}秒まで）", { maxReviewSeconds })}
            <input
              key={fileKey}
              type="file"
              accept="video/*"
              className="sr-only"
              onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
            />
          </label>
        ) : null}
        {isCoach && !menu ? (
          <button type="button" className="text-blue-300 underline" onClick={() => setMenu(emptyMenu())}>{L("メニュー作成")}</button>
        ) : null}
      </div>
      {upload ? <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} /> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || (!body.trim() && !file && !menu)}>{L("送る")}</Button>
      {guard.warning}
      <ContactNotice />
    </form>
  );
}

// 返答の目安（全コーチ一律、強制の期限ではない）。
export const replyGuideline = "コーチからの返答は24時間以内を目安としています。";
export const coachReplyGuideline = "選手への返答は24時間以内を目安にしてください（目安で、期限ではありません）。";

// 期間中に出す、終わったら見られなくなることの案内。保護者はダウンロードできないので、ダウンロードの案内は出さない。
function closeNote(until: string | null | undefined, guardian: boolean): string {
  const when = until ? L("{v0}に契約が終わると", { v0: formatDateTime(until) }) : L("契約が終わると");
  return L("{when}、このやりとりは見られなくなります（選手・コーチ・保護者とも）。{v1}", { when, v1: guardian ? "" : L("必要なら、終わる前に文章とメニューをダウンロードしてください。") });
}

// DownloadChat は、やりとり（文章とメニュー）をテキストで保存する。動画は含めない。
function DownloadChat({ contract, token }: { contract: CoachingContract; token: string | null }) {
  const [error, setError] = useState("");
  async function download() {
    setError("");
    try {
      const text = await apiDownloadText(`/v1/coaching/contracts/${contract.id}/messages/export`, token);
      const url = URL.createObjectURL(new Blob([text], { type: "text/plain;charset=utf-8" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = `tracktalk-chat-${contract.id.slice(0, 8)}.txt`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : L("ダウンロードできませんでした"));
    }
  }
  return (
    <div className="space-y-1">
      <Button type="button" variant="secondary" className="w-full" onClick={() => void download()}>{L("やりとり（文章とメニュー）をダウンロード")}</Button>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

// CoachingMessages は、期間中・止めている間・終わったあとの契約に出すやりとり欄。
// guardianToken を渡すと、保護者のページとしてリンクのトークンで読み書きする。
export function CoachingMessages({
  contract,
  token,
  guardianToken,
}: {
  contract: CoachingContract;
  token: string | null;
  guardianToken?: string;
}) {
  const [messages, setMessages] = useState<CoachingMessage[] | null>(null);
  const [error, setError] = useState("");
  // 読み込みが重なったり、送信した直後に始まった読み込みが返ったりしても、古い一覧で上書きしないよう、
  // 最新の読み込みだけ反映する（送信したら、それまでに始まった読み込みを無効にする）。
  const loadSeq = useRef(0);
  const path = contractPath(contract, { token, guardianToken });

  const [viewClosed, setViewClosed] = useState(false);
  const fetchMessages = useCallback(() => apiFetch<{ messages: CoachingMessage[]; view_closed?: boolean }>(`${path}/messages`, { token }), [path, token]);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      const seq = ++loadSeq.current;
      fetchMessages()
        .then((d) => {
          if (cancelled || seq !== loadSeq.current) return;
          setMessages(d.messages);
          setViewClosed(Boolean(d.view_closed));
          setError("");
        })
        .catch(() => {
          if (!cancelled && seq === loadSeq.current) setError(L("やりとりを読み込めませんでした"));
        });
    };
    load();
    if (contract.status !== "active") {
      return () => {
        cancelled = true;
      };
    }
    const timer = window.setInterval(load, 30000);
    const refresh = () => {
      if (document.visibilityState === "visible") load();
    };
    document.addEventListener("visibilitychange", refresh);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [fetchMessages, contract.status]);

  const access: Access = { token, guardianToken };
  if (contract.messages_purged_at) {
    return (
      <div className="space-y-4">
        <p className="rounded-xl border border-white/20 bg-white/5 p-3 text-sm text-slate-200">{L("期間の終わりから1年たったので、やりとり（文章と動画）は消しました。")}</p>
      </div>
    );
  }
  const isGuardian = Boolean(guardianToken) || contract.role === "guardian";
  const ended = contract.status === "ended" || contract.status === "refunded";
  // 終わったあとに読めるのは、通報・紛争の対応中で保全しているときだけ。
  const preservedNote = L("通報・紛争の対応中のため、対応が終わるまで読むことだけができます。");
  return (
    <div className="space-y-4">
      <p className="rounded-xl border border-white/20 bg-white/5 p-3 text-xs text-slate-200">
        {isGuardian
          ? L("保護者のページでは、やりとりを見ることだけができます（送信はできません）。気になるやりとりは通報できます。")
          : contract.minor
            ? L("保護者もやりとりを見ることができます（保護者は送信できません）。通報があったときだけ、運営が確認します。")
            : L("このやりとりは、あなたと相手だけが見られます。通報があったときだけ、運営が確認します。")}
        {contract.status === "active" ? ` ${contract.role === "coach" ? L(coachReplyGuideline) : L(replyGuideline)}` : ""}
        {contract.status === "active" || contract.status === "suspended" ? ` ${closeNote(contract.messages_view_until, isGuardian)}` : ""}
      </p>
      {viewClosed ? (
        <p className="rounded-xl border border-white/20 bg-white/5 p-3 text-sm text-slate-200">{L("契約が終わったので、このやりとりは見られません。")}</p>
      ) : null}
      <section className="space-y-2" aria-label={L("やりとり")}>
        {messages === null && !error ? <PageLoading label={L("読み込み中...")} /> : null}
        {messages?.length === 0 ? (
          <p className="text-sm text-slate-300">
            {contract.role === "coach"
              ? L("まずはあいさつと、最初に送ってほしい動画を伝えましょう。")
              : contract.role === "guardian"
                ? L("まだやりとりはありません。")
                : L("まずはあいさつと、いまの練習の様子を送りましょう。")}
          </p>
        ) : null}
        <ul className="space-y-3">
          {messages?.map((m) => (
            <MessageItem key={m.id} message={m} contract={contract} access={access} />
          ))}
        </ul>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </section>
      {contract.status === "active" && !coachingMessagingEnabled ? (
        <p className="text-sm text-slate-300">{L(messagingClosedNote)}</p>
      ) : viewClosed ? null : contract.status === "active" && isGuardian ? null : contract.status === "active" ? (
        <Composer contract={contract} access={access} onSent={(m) => {
            loadSeq.current++;
            setMessages((list) => [...(list ?? []), m]);
          }} />
      ) : contract.status === "suspended" ? (
        <p className="rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm text-amber-100">{L("運営が確認のため、やりとりを止めています。{v0}", { v0: contract.fulfillment_version === 1 ? L("大会日までは延長せず、未提供分は回数に応じて返金します。") : L("止めている日数の分、期間はあとで延びます。") })}</p>
      ) : ended ? (
        <p className="text-sm text-slate-300">{contract.status === "refunded" ? L("返金して終了しました。") : L("期間が終わりました。")}{preservedNote}</p>
      ) : null}
      {!viewClosed && !isGuardian && messages && messages.length > 0 ? <DownloadChat contract={contract} token={token} /> : null}
    </div>
  );
}
