"use client";

import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError, apiErrorText } from "@/lib/api";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { formatDate, formatDateTime } from "@/lib/format";
import { maxReviewSeconds, tooLong, videoDuration } from "@/lib/review";
import type { CoachingContract, CoachingMessage, PlanMenu } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";

// パーソナル指導のやりとり欄。期間中は文章と動画を送れ、コーチはメニューも送れる。終わったあとは読むだけ。
// 18歳未満のときは保護者もリンクから参加する（保護者は文章だけ）。相手の1通ごとに通報できる。
// リアルタイムではないので、開いている間は30秒ごとと、画面に戻ったときに読み直す。

const maxVideoBytes = 100 * 1024 * 1024;
const maxBodyChars = 2000;

function senderName(m: CoachingMessage, contract: CoachingContract): string {
  if (m.mine) return "あなた";
  if (!m.sender_active) return "退会したユーザー";
  if (m.sender_role === "coach") return contract.coach?.nickname ?? "コーチ";
  if (m.sender_role === "guardian") return "保護者";
  return `${contract.requester?.nickname ?? "名前なし"}さん`;
}

function MenuCard({ menu }: { menu: PlanMenu }) {
  return (
    <div className="space-y-1.5 rounded-lg border border-blue-400/40 bg-blue-500/10 p-2">
      <p className="text-xs font-semibold text-blue-300">練習メニュー</p>
      <p className="font-semibold text-white">{menu.title}</p>
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
    if (!(await confirmDialog(reportConfirmOptions({ title: "このメッセージを運営に通報しますか？", body: "通報すると、運営がこのやりとりを確認します。" })))) return;
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

  if (state === "done") return <p className="text-xs text-slate-300">通報しました。運営が確認します。</p>;
  return (
    <button type="button" onClick={() => void report()} disabled={pending} className="text-xs text-slate-300 underline">
      {state === "error" ? "通報できませんでした。もう一度" : "通報する"}
    </button>
  );
}

function MessageItem({ message, contract, access }: { message: CoachingMessage; contract: CoachingContract; access: Access }) {
  return (
    <li className={`flex ${message.mine ? "justify-end" : "justify-start"}`}>
      <div
        className={`max-w-[85%] space-y-2 rounded-2xl p-3 text-sm ${
          message.mine ? "bg-blue-600/60 text-white" : "border border-white/20 bg-white/10 text-slate-100"
        }`}
      >
        <p className="text-xs text-slate-300">
          {senderName(message, contract)} · {formatDateTime(message.created_at)}
        </p>
        {message.body ? <p className="whitespace-pre-wrap">{message.body}</p> : null}
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
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [menu, setMenu] = useState<PlanMenu | null>(null);
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [error, setError] = useState("");
  const [fileKey, setFileKey] = useState(0);
  const { pending, acquire, release } = useActionLock();
  const isCoach = contract.role === "coach";

  // 長さを確かめ終えてから選んだことにする(確かめているあいだに送れないように)。
  // 続けて別の動画を選んだら、前の動画の確認の結果は使わない。
  const pickSeq = useRef(0);
  async function onPickFile(picked: File | null) {
    const seq = ++pickSeq.current;
    setError("");
    setFile(null);
    if (!picked) return;
    if (picked.size > maxVideoBytes) {
      setError("100MB以内の動画にしてください");
      return;
    }
    const long = tooLong(await videoDuration(picked));
    if (seq !== pickSeq.current) return;
    if (long) {
      setError(`動画は${maxReviewSeconds}秒以内にしてください`);
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
    if (!acquire()) return;
    setError("");
    try {
      let videoPublicID = "";
      if (file) {
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/coaching-video", { method: "POST", token });
        setUploadPhase("upload");
        setUpload({ loaded: 0, total: file.size, percent: 0 });
        const payload = await uploadToCloudinary("video", file, signed, setUpload);
        if (payload.public_id !== signed.public_id) throw new Error("アップロードに失敗しました");
        videoPublicID = payload.public_id;
        setUploadPhase("saving");
      }
      const d = await apiFetch<{ message: CoachingMessage }>(`${contractPath(contract, access)}/messages`, {
        method: "POST",
        token,
        body: JSON.stringify({ body, video_public_id: videoPublicID, menu }),
      });
      onSent(d.message);
      setBody("");
      setFile(null);
      setFileKey((k) => k + 1);
      setMenu(null);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : "送信に失敗しました"));
    } finally {
      setUpload(null);
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-3">
      <Textarea
        rows={3}
        value={body}
        maxLength={maxBodyChars}
        onChange={(e) => setBody(e.target.value)}
        placeholder={
          isCoach
            ? "練習の様子への返事や、次にやることを書きます"
            : access.guardianToken
              ? "コーチへの質問や、家での様子を書きます"
              : "練習の様子や、聞きたいことを書きます"
        }
        aria-label="メッセージ"
      />
      {menu ? (
        <div className="space-y-2 rounded-xl border border-blue-400/40 bg-blue-500/10 p-2">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold text-blue-300">練習メニュー</p>
            <button type="button" className="text-xs text-slate-300 underline" onClick={() => setMenu(null)}>
              メニューをやめる
            </button>
          </div>
          <Input
            value={menu.title}
            maxLength={60}
            onChange={(e) => setMenu({ ...menu, title: e.target.value })}
            placeholder="例: 1週目 スタートの練習"
            aria-label="メニューの名前"
          />
          {menu.days.map((d, i) => (
            <div key={i} className="grid grid-cols-[5rem_1fr] gap-2">
              <Input value={d.day} maxLength={20} onChange={(e) => setDay(i, "day", e.target.value)} placeholder="月" aria-label={`${i + 1}日目の日付や曜日`} />
              <Textarea
                rows={2}
                value={d.content}
                maxLength={300}
                onChange={(e) => setDay(i, "content", e.target.value)}
                placeholder="例: 流し 100m×5、スタート 30m×6"
                aria-label={`${i + 1}日目の中身`}
              />
            </div>
          ))}
          {menu.days.length < 14 ? (
            <button
              type="button"
              className="text-xs text-blue-300 underline"
              onClick={() => setMenu({ ...menu, days: [...menu.days, { day: "", content: "" }] })}
            >
              1日足す
            </button>
          ) : null}
        </div>
      ) : null}
      <div className="flex flex-wrap items-center gap-3 text-sm">
        {canVideo ? (
          <label className="cursor-pointer text-blue-300 underline">
            {file ? `動画: ${file.name}` : `動画をつける（${maxReviewSeconds}秒まで）`}
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
          <button type="button" className="text-blue-300 underline" onClick={() => setMenu(emptyMenu())}>
            メニューをつける
          </button>
        ) : null}
      </div>
      {upload ? <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} /> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || (!body.trim() && !file && !menu)}>
        送る
      </Button>
      <p className="text-xs text-slate-300">連絡先（URL・SNS の ID・電話番号など）は送れません。</p>
    </form>
  );
}

function RatingForm({ contract, token, onRated }: { contract: CoachingContract; token: string | null; onRated: (stars: number) => void }) {
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const stars = contract.rating?.stars ?? 0;

  async function rate(n: number) {
    if (contract.rating || !acquire()) return;
    setError("");
    try {
      await apiFetch(`/v1/coaching/contracts/${contract.id}/rating`, { method: "POST", token, body: JSON.stringify({ stars: n }) });
      onRated(n);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  return (
    <section className="space-y-2">
      <h2 className="text-sm font-semibold">このパーソナル指導の評価</h2>
      <p className="text-xs text-slate-300">{contract.rating ? "評価しました。ありがとうございました。" : "★を選ぶと送られます。1回だけです。"}</p>
      <div className="flex gap-1" role="radiogroup" aria-label="評価">
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={stars === n}
            aria-label={`${n}つ星`}
            disabled={pending || !!contract.rating}
            onClick={() => void rate(n)}
            className={`text-3xl leading-none transition active:scale-[0.98] disabled:opacity-80 ${n <= stars ? "text-amber-300" : "text-white/25"}`}
          >
            ★
          </button>
        ))}
      </div>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}

// やりとりは、期間の終わりから1年たつと文章も動画も消える（未解決の通報があるものは運営の対応が終わるまで残る）。
function purgeDate(endsAt: string): string {
  const d = new Date(endsAt);
  d.setFullYear(d.getFullYear() + 1);
  return formatDate(d.toISOString());
}

// CoachingMessages は、期間中・止めている間・終わったあとの契約に出すやりとり欄。
// guardianToken を渡すと、保護者のページとしてリンクのトークンで読み書きする。
export function CoachingMessages({
  contract,
  token,
  guardianToken,
  onContractChange,
}: {
  contract: CoachingContract;
  token: string | null;
  guardianToken?: string;
  onContractChange: (c: CoachingContract) => void;
}) {
  const [messages, setMessages] = useState<CoachingMessage[] | null>(null);
  const [error, setError] = useState("");
  // 読み込みが重なったり、送信した直後に始まった読み込みが返ったりしても、古い一覧で上書きしないよう、
  // 最新の読み込みだけ反映する（送信したら、それまでに始まった読み込みを無効にする）。
  const loadSeq = useRef(0);
  const path = contractPath(contract, { token, guardianToken });

  const fetchMessages = useCallback(() => apiFetch<{ messages: CoachingMessage[] }>(`${path}/messages`, { token }), [path, token]);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      const seq = ++loadSeq.current;
      fetchMessages()
        .then((d) => {
          if (cancelled || seq !== loadSeq.current) return;
          setMessages(d.messages);
          setError("");
        })
        .catch(() => {
          if (!cancelled && seq === loadSeq.current) setError("やりとりを読み込めませんでした");
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
        <p className="rounded-xl border border-white/20 bg-white/5 p-3 text-sm text-slate-200">
          期間の終わりから1年たったので、やりとり（文章と動画）は消しました。
        </p>
        {contract.status === "ended" && contract.role === "requester" ? (
          <RatingForm
            contract={contract}
            token={token}
            onRated={(stars) => onContractChange({ ...contract, rating: { stars, rated_at: new Date().toISOString() } })}
          />
        ) : null}
      </div>
    );
  }
  const readOnlyNote = contract.ends_at ? `${purgeDate(contract.ends_at)}ごろに、やりとりは消えます。` : "";
  return (
    <div className="space-y-4">
      <p className="rounded-xl border border-white/20 bg-white/5 p-3 text-xs text-slate-200">
        {contract.minor
          ? "保護者も参加しています。通報があったときだけ、運営が確認します。"
          : "このやりとりは、あなたと相手だけが見られます。通報があったときだけ、運営が確認します。"}
      </p>
      <section className="space-y-2" aria-label="やりとり">
        {messages === null && !error ? <p className="text-sm text-slate-200">読み込み中...</p> : null}
        {messages?.length === 0 ? (
          <p className="text-sm text-slate-300">
            {contract.role === "coach"
              ? "まずはあいさつと、最初に送ってほしい動画を伝えましょう。"
              : contract.role === "guardian"
                ? "まだやりとりはありません。"
                : "まずはあいさつと、いまの練習の様子を送りましょう。"}
          </p>
        ) : null}
        <ul className="space-y-3">
          {messages?.map((m) => (
            <MessageItem key={m.id} message={m} contract={contract} access={access} />
          ))}
        </ul>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </section>
      {contract.status === "active" ? (
        <Composer contract={contract} access={access} onSent={(m) => {
            loadSeq.current++;
            setMessages((list) => [...(list ?? []), m]);
          }} />
      ) : contract.status === "suspended" ? (
        <p className="rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm text-amber-100">
          運営が確認のため、やりとりを止めています。止めている日数の分、期間はあとで延びます。
        </p>
      ) : contract.status === "refunded" ? (
        <p className="text-sm text-slate-300">返金して終了したので、読むだけになりました。{readOnlyNote}</p>
      ) : (
        <p className="text-sm text-slate-300">期間が終わったので、読むだけになりました。{readOnlyNote}</p>
      )}
      {contract.status === "ended" && contract.role === "requester" ? (
        <RatingForm
          contract={contract}
          token={token}
          onRated={(stars) => onContractChange({ ...contract, rating: { stars, rated_at: new Date().toISOString() } })}
        />
      ) : null}
    </div>
  );
}
