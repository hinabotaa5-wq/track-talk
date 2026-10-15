"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { Comment, Meta, Post } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import { feedTitle } from "@/components/feed-card";
import { DetailLoadingFrame } from "@/components/loading-frame";
import { PracticeReputationSummary } from "@/components/practice-reputation";
import { prefectureName } from "@/lib/prefectures";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { canCommentOnPost } from "@/lib/practice-comments";
import { buildCommentThreads, replyLabel } from "@/lib/comment-thread";
import { commentSchema } from "@/lib/validators";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import Link from "next/link";
import { ZoomableImage } from "@/components/zoomable-image";
import { ZoomableVideo } from "@/components/zoomable-video";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { YouTubePlayer } from "@/components/youtube-player";
import { LikeButton, ReportButton, SaveButton } from "@/components/action-button";
import { InjuryRecovery } from "@/components/injury-recovery";
import { InjuryTypicalRecovery } from "@/components/injury-typical-recovery";
import { clearHomeFeeds, clearSavedFeeds } from "@/lib/home-feed-cache";

export function PostDetail({ id, returnTo }: { id: string; returnTo?: "me" }) {
  const { session, token } = useAuth();
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [error, setError] = useState("");
  const [commentError, setCommentError] = useState("");
  const [loading, setLoading] = useState(true);
  const form = useForm({ resolver: zodResolver(commentSchema), defaultValues: { body: "" } });
  const action = useActionLock();
  const commentLock = useActionLock();
  const [replyTo, setReplyTo] = useState<Comment | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(null);
  const [reportedPost, setReportedPost] = useState(false);
  const [reportedComments, setReportedComments] = useState<Set<string>>(new Set());

  async function load() {
    try {
      const [p, c] = await Promise.all([
        apiFetch<Post>(`/v1/posts/${id}`, { token }),
        apiFetch<{ comments: Comment[]; meta: Meta }>(`/v1/posts/${id}/comments?per_page=50`, { token }),
      ]);
      setPost(p);
      setComments(c.comments ?? []);
      if (token) {
        const mine = await apiFetch<{ post: boolean; comment_ids: string[] }>(`/v1/posts/${id}/my-reports`, { token }).catch(() => null);
        if (mine) {
          setReportedPost(mine.post);
          setReportedComments(new Set(mine.comment_ids));
        }
      }
      setError("");
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 404) setError("投稿が見つかりません");
      else if (e instanceof ApiRequestError && (e.status === 401 || e.code === "not_joined")) {
        // ケガの投稿は、同じ部位のグループに参加した人だけが読める。
        setError("この投稿は、同じ部位のグループに参加すると読めます");
      } else setError("読み込みに失敗しました");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => () => document.body.removeAttribute("data-comment-typing"), []);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      void load();
    }, 0);
    function onVisible() {
      if (document.visibilityState === "visible") void load();
    }
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearTimeout(handle);
      document.removeEventListener("visibilitychange", onVisible);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, token]);

  if (loading) return <DetailLoadingFrame />;
  if (error || !post) return <p className="text-red-400">{error || "エラー"}</p>;

  const current = post;
  const mine = current.mine;
  const allowComment = canCommentOnPost(current);

  async function toggleLike() {
    if (!session) {
      router.push("/login");
      return;
    }
    if (!action.acquire()) return;
    try {
      await apiFetch(`/v1/posts/${current.id}/likes`, { method: current.liked_by_me ? "DELETE" : "POST", token });
      clearHomeFeeds();
      await load();
    } finally {
      action.release();
    }
  }

  async function toggleSave() {
    if (!session) {
      router.push("/login");
      return;
    }
    if (!action.acquire()) return;
    try {
      await apiFetch(`/v1/posts/${current.id}/saves`, { method: current.saved_by_me ? "DELETE" : "POST", token });
      await load();
    } finally {
      action.release();
    }
  }

  async function remove() {
    if (!confirm("この投稿を削除しますか？")) return;
    if (!action.acquire()) return;
    try {
      await apiFetch(`/v1/posts/${current.id}`, { method: "DELETE", token });
    } catch (e) {
      action.release();
      alert(e instanceof ApiRequestError ? e.message : "削除できませんでした");
      return;
    }
    clearHomeFeeds();
    clearSavedFeeds();
    router.push(returnTo === "me" ? "/me" : current.kind === "injury" ? "/injury" : "/");
    router.refresh();
  }

  function markReported(targetType: "post" | "comment", targetId: string) {
    if (targetType === "post") setReportedPost(true);
    else setReportedComments((prev) => new Set(prev).add(targetId));
  }

  async function report(targetType: "post" | "comment", targetId: string) {
    if (!session) {
      router.push("/login");
      return;
    }
    if (!confirm("この内容を通報しますか？")) return;
    if (!action.acquire()) return;
    try {
      await apiFetch("/v1/reports", {
        method: "POST",
        token,
        body: JSON.stringify({
          target_type: targetType,
          target_id: targetId,
          reason: "other",
          detail: "",
        }),
      });
      markReported(targetType, targetId);
      alert("通報を受け付けました");
    } catch (e) {
      if (e instanceof ApiRequestError && e.status === 409) {
        markReported(targetType, targetId);
        alert("すでに通報済みです");
      } else alert(e instanceof ApiRequestError ? e.message : "通報できませんでした");
    } finally {
      action.release();
    }
  }

  async function onComment(values: { body: string }) {
    if (!session) {
      router.push("/login");
      return;
    }
    if (!commentLock.acquire()) return;
    setCommentError("");
    try {
      await apiFetch(`/v1/posts/${current.id}/comments`, {
        method: "POST",
        token,
        body: JSON.stringify(replyTo ? { ...values, reply_to_id: replyTo.id } : values),
      });
      form.reset();
      setReplyTo(null);
      await load();
    } catch (e) {
      setCommentError(e instanceof ApiRequestError ? e.message : "コメントできませんでした");
      await load();
    } finally {
      commentLock.release();
    }
  }

  // 返信先コメントの投稿者名(API が「あなた」「投稿者」「ニックネーム/匿名」に解決済み)。
  function replyTargetName(c: Comment) {
    return c.author_name ?? "匿名";
  }

  function startReply(c: Comment) {
    setReplyTo(c);
    form.setFocus("body");
  }

  function jumpToComment(id: string) {
    const el = document.getElementById(`comment-${id}`);
    if (!el) return;
    el.scrollIntoView({ behavior: "smooth", block: "center" });
    setHighlightId(id);
    window.setTimeout(() => setHighlightId((cur) => (cur === id ? null : cur)), 1600);
  }

  async function editComment(c: Comment) {
    const body = window.prompt("コメントを編集", c.body);
    if (!body) return;
    if (!action.acquire()) return;
    try {
      await apiFetch(`/v1/comments/${c.id}`, {
        method: "PATCH",
        token,
        body: JSON.stringify({ body }),
      });
      await load();
    } finally {
      action.release();
    }
  }

  async function deleteComment(cid: string) {
    if (!action.acquire()) return;
    try {
      await apiFetch(`/v1/comments/${cid}`, { method: "DELETE", token });
      await load();
    } catch (e) {
      alert(e instanceof ApiRequestError ? e.message : "削除できませんでした");
    } finally {
      action.release();
    }
  }

  function renderComment(c: Comment, isReply: boolean) {
    return (
      <div
        key={c.id}
        id={`comment-${c.id}`}
        className={`rounded-xl border glass p-3.5 text-slate-200 transition-colors ${highlightId === c.id ? "border-blue-400 bg-blue-400/20" : "border-white/20 bg-white/10"}`}
      >
        {isReply && c.reply_to_id ? (
          <button
            type="button"
            className="mb-1 inline-flex max-w-full items-center gap-1 rounded-full bg-blue-400/20 px-2 py-0.5 text-xs font-semibold text-blue-200 disabled:opacity-60"
            disabled={c.reply_to_removed}
            onClick={() => jumpToComment(c.reply_to_id as string)}
          >
            <span aria-hidden>↩</span>
            <span className="truncate">@{replyLabel(c)}</span>
          </button>
        ) : null}
        <p className="mb-0.5 text-xs font-semibold text-slate-100">{c.author_name ?? "匿名"}</p>
        <p className="whitespace-pre-wrap text-sm">{c.body}</p>
        <p className="mt-2 text-xs text-slate-300">{formatDateTime(c.created_at)}</p>
        <div className="mt-1 flex flex-wrap items-center gap-3">
          {allowComment ? (
            <button
              type="button"
              className="text-xs font-semibold text-slate-100 transition active:scale-95 disabled:opacity-50"
              disabled={action.pending}
              onClick={() => startReply(c)}
            >
              返信
            </button>
          ) : null}
          {c.mine ? (
            <>
              <button
                type="button"
                className="text-xs text-blue-400 transition active:scale-95 disabled:opacity-50"
                disabled={action.pending}
                onClick={() => editComment(c)}
              >
                編集
              </button>
              <button
                type="button"
                className="text-xs text-red-400 transition active:scale-95 disabled:opacity-50"
                disabled={action.pending}
                onClick={() => deleteComment(c.id)}
              >
                削除
              </button>
            </>
          ) : (
            <button
              type="button"
              className={
                reportedComments.has(c.id)
                  ? "inline-flex items-center gap-1 text-xs font-semibold text-amber-300"
                  : "text-xs text-slate-300 transition active:scale-95 disabled:opacity-50"
              }
              disabled={action.pending || reportedComments.has(c.id)}
              onClick={() => report("comment", c.id)}
            >
              {reportedComments.has(c.id) ? "通報済み" : "通報"}
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <article
      className={`${allowComment ? "pb-[calc(12.5rem+env(safe-area-inset-bottom))] md:pb-36 lg:pb-0" : ""} lg:grid lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start lg:gap-8`}
    >
      <div className="space-y-6">
      {post.kind === "practice" ? (
        <h1 className="sr-only">{post.practice?.event_label || "練習相手募集"}</h1>
      ) : null}
      {post.kind === "injury" && post.injury ? (
        <p className="flex flex-wrap items-center gap-2">
          <span className="rounded-full bg-white/12 px-3 py-1 text-xs font-semibold text-slate-200 border border-white/20">
            {post.injury.body_part_label}
          </span>
          {post.injury.stage_label ? (
            <span className="rounded-full bg-emerald-950/70 px-3 py-1 text-xs font-bold text-emerald-300 border border-emerald-800/60">
              {post.injury.stage_label}
            </span>
          ) : null}
        </p>
      ) : null}
      {post.kind === "recruitment" ? (
        <RecruitmentFacts post={post} />
      ) : post.kind === "practice" ? (
        <>
          <PracticeFacts post={post} />
          <PracticeJoin post={post} onDone={load} />
        </>
      ) : post.kind === "video" && post.playback_url ? (
        <ZoomableVideo
          className="aspect-video w-full bg-black"
          src={post.playback_url}
          poster={post.thumbnail_url}
        />
      ) : post.kind === "youtube" && post.youtube_video_id ? (
        <YouTubePlayer
          videoId={post.youtube_video_id}
          title={feedTitle(post.title)}
          thumbnailUrl={post.thumbnail_url}
        />
      ) : post.thumbnail_url ? (
        <ZoomableImage
          src={post.thumbnail_url}
          alt={feedTitle(post.title)}
          sizes="(max-width: 1024px) 100vw, 800px"
          wrapperClassName="relative w-full overflow-hidden rounded-xl bg-black/25"
          imageClassName="max-h-[70vh] w-full object-contain h-auto"
          priority
        />
      ) : null}
      {post.kind === "practice" ? null : (
        <h1 className="text-3xl font-bold">{feedTitle(post.title)}</h1>
      )}
      <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-slate-200">
        <span>{formatDateTime(post.created_at)}</span>
        {post.author_name ? <span>{post.author_name}</span> : null}
      </p>
      {(post.kind === "recruitment" || post.kind === "practice") && post.body ? (
        <div>
          <h2 className="mb-2 text-xl font-semibold text-slate-100">詳細</h2>
          <div className="whitespace-pre-wrap rounded-xl border border-white/20 glass bg-white/10 p-4 leading-7 text-slate-200">{post.body}</div>
        </div>
      ) : post.body ? (
        <div className="whitespace-pre-wrap rounded-xl border border-white/20 glass bg-white/10 p-4 leading-7 text-slate-200">{post.body}</div>
      ) : null}
      {post.kind === "injury" ? (
        <InjuryRecovery postId={post.id} updates={post.injury?.updates ?? []} mine={post.mine} onChanged={load} />
      ) : null}
      {post.kind === "injury" && post.injury ? (
        <InjuryTypicalRecovery bodyPart={post.injury.body_part} bodyPartLabel={post.injury.body_part_label} />
      ) : null}
      <div className="flex flex-wrap items-center gap-2.5 pt-2">
        <LikeButton
          liked={post.liked_by_me}
          count={post.like_count}
          disabled={action.pending}
          onClick={toggleLike}
          label={post.kind === "injury" ? "わかる" : undefined}
        />
        <SaveButton
          saved={post.saved_by_me}
          disabled={action.pending}
          onClick={toggleSave}
        />
        {mine ? (
          <>
            <Link
              href={returnTo === "me" ? `/posts/${post.id}/edit?from=me` : `/posts/${post.id}/edit`}
              aria-disabled={action.pending}
              className="inline-flex items-center justify-center rounded-full border border-white/25 glass bg-white/12 px-4 py-2 text-sm font-semibold text-slate-200 shadow-xs transition hover:bg-white/16 active:scale-95 aria-disabled:pointer-events-none aria-disabled:opacity-50"
            >
              編集
            </Link>
            <Button
              type="button"
              variant="danger"
              className="rounded-full shadow-xs active:scale-95"
              disabled={action.pending}
              onClick={remove}
            >
              削除
            </Button>
          </>
        ) : null}
        <ReportButton reported={reportedPost} disabled={action.pending} onClick={() => report("post", post.id)} />
      </div>
      </div>
      <section className="mt-6 space-y-3 lg:sticky lg:top-4 lg:mt-0">
        <h2 className="text-xl font-semibold">コメント</h2>
        {comments.length === 0 ? <p className="text-sm text-slate-200">まだコメントはありません。</p> : null}
        {buildCommentThreads(comments).map((thread) => (
          <div key={thread.top?.id ?? thread.replies[0].id} className="space-y-2">
            {thread.top ? (
              renderComment(thread.top, false)
            ) : (
              <p className="rounded-xl border border-dashed border-white/20 p-3 text-xs text-slate-300">削除されたコメント</p>
            )}
            {thread.replies.length > 0 ? (
              <div className="ml-4 space-y-2 border-l-2 border-blue-400/40 pl-3">{thread.replies.map((r) => renderComment(r, true))}</div>
            ) : null}
          </div>
        ))}
        {allowComment ? (
          <form
            onSubmit={form.handleSubmit(onComment)}
            onFocus={() => document.body.setAttribute("data-comment-typing", "")}
            onBlur={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                document.body.removeAttribute("data-comment-typing");
              }
            }}
            className="glass fixed inset-x-0 bottom-[calc(3.5rem+env(safe-area-inset-bottom))] [body[data-comment-typing]_&]:bottom-0 z-20 border-t border-white/20 bg-white/10 p-3 md:bottom-0 md:pb-[max(0.75rem,env(safe-area-inset-bottom))] lg:static lg:bottom-auto lg:rounded-xl lg:border lg:p-3 lg:pb-3"
          >
            {replyTo ? (
              <div className="mx-auto mb-2 flex w-full max-w-5xl items-center gap-2 rounded-lg bg-blue-400/20 px-3 py-1.5 text-xs text-blue-100 lg:mx-0 lg:max-w-none">
                <span className="min-w-0 flex-1 truncate">
                  <span className="font-semibold">@{replyTargetName(replyTo)}</span> に返信中: {replyTo.body}
                </span>
                <button type="button" aria-label="返信をやめる" className="shrink-0 px-1 text-base leading-none" onClick={() => setReplyTo(null)}>
                  ×
                </button>
              </div>
            ) : null}
            <div className="mx-auto flex w-full max-w-5xl items-end gap-2 lg:mx-0 lg:max-w-none lg:flex-col lg:items-stretch">
              <Textarea rows={2} className="min-h-11 flex-1 glass bg-white/8 border-white/20 text-white placeholder:text-slate-300 focus:border-blue-500" placeholder="コメントを書く" {...form.register("body")} />
              <Button type="submit" className="shrink-0 lg:w-full" disabled={commentLock.pending}>
                {commentLock.pending ? "送っています" : "コメントする"}
              </Button>
            </div>
            {commentError ? <p className="mt-2 text-sm text-red-400">{commentError}</p> : null}
          </form>
        ) : (
          <p className="rounded-xl border border-white/20 glass bg-white/10 p-3.5 text-sm text-slate-200">募集は終了しています。</p>
        )}
      </section>
    </article>
  );
}

function practiceStatusLabel(status: "open" | "full" | "dissolved") {
  if (status === "full") return "満員";
  if (status === "dissolved") return "解散";
  return "募集中";
}

function applicationStatusLabel(status: "pending" | "accepted" | "declined") {
  if (status === "accepted") return "承認済み";
  if (status === "declined") return "見送り";
  return "申請中";
}

function RatingForm({
  disabled,
  onSubmit,
}: {
  disabled: boolean;
  onSubmit: (score: string, comment: string) => void;
}) {
  const [score, setScore] = useState("");
  const [comment, setComment] = useState("");
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        if (!score) return;
        if (!confirm("この評価は変更できません。送信しますか？")) return;
        onSubmit(score, comment);
      }}
      className="mt-2 space-y-2"
    >
      <label className="block text-sm font-medium">
        評価
        <select
          className="mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
          value={score}
          onChange={(event) => setScore(event.target.value)}
        >
          <option value="">選んでください</option>
          <option value="good">良い</option>
          <option value="ok">普通</option>
          <option value="bad">悪い</option>
        </select>
      </label>
      <Textarea
        rows={2}
        maxLength={200}
        placeholder="ひとこと（任意）"
        aria-label="評価コメント"
        value={comment}
        onChange={(event) => setComment(event.target.value)}
      />
      <Button type="submit" disabled={disabled || score === ""}>
        評価を送る
      </Button>
    </form>
  );
}

function PracticeJoin({ post, onDone }: { post: Post; onDone: () => Promise<void> }) {
  const { session, token } = useAuth();
  const router = useRouter();
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  const info = post.practice;
  if (!info) return null;

  async function run(path: string, confirmMessage?: string) {
    if (!session) {
      router.push("/login");
      return;
    }
    if (confirmMessage && !confirm(confirmMessage)) return;
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(path, { method: "POST", token });
      await onDone();
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "処理に失敗しました");
    } finally {
      release();
    }
  }

  async function rate(applicationId: string, score: string, comment: string) {
    if (!session) {
      router.push("/login");
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(`/v1/posts/${post.id}/practice/applications/${applicationId}/rating`, {
        method: "POST",
        token,
        body: JSON.stringify({ score, comment }),
      });
      await onDone();
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "処理に失敗しました");
    } finally {
      release();
    }
  }

  const mine = post.mine;
  const open = info.status === "open";

  return (
    <div className="space-y-3">
      {mine && info.status !== "dissolved" ? (
        <Button type="button" variant="danger" disabled={pending} onClick={() => run(`/v1/posts/${post.id}/practice/dissolve`, "この練習を解散しますか？")}>
          解散する
        </Button>
      ) : null}
      {!mine && info.my_application === "pending" ? <p className="text-sm">申請中です</p> : null}
      {!mine && info.my_application === "accepted" ? <p className="text-sm">参加が承認されました</p> : null}
      {!mine && info.my_application === "declined" ? <p className="text-sm">見送りになりました</p> : null}
      {!mine && !info.my_application && info.status === "full" ? <p className="text-sm">満員です</p> : null}
      {!mine && !info.my_application && open ? (
        <Button type="button" disabled={pending} onClick={() => run(`/v1/posts/${post.id}/practice/applications`)}>
          参加したい
        </Button>
      ) : null}
      {!mine && info.status === "dissolved" && info.my_application === "accepted" && info.my_application_id ? (
        info.rated_host ? (
          <p className="text-sm">評価しました</p>
        ) : (
          <div>
            <p className="text-sm font-medium">募集者を評価</p>
            <RatingForm disabled={pending} onSubmit={(score, comment) => rate(info.my_application_id ?? "", score, comment)} />
          </div>
        )
      ) : null}
      {mine && info.applications && info.applications.length > 0 ? (
        <ul className="space-y-2">
          {info.applications.map((application) => (
            <li key={application.id} className="rounded-xl border border-white/20 glass bg-white/10 p-3">
              <p className="text-sm">参加希望 · {formatDateTime(application.created_at)}</p>
              <p className="mt-1 text-sm text-slate-200">{applicationStatusLabel(application.status)}</p>
              {application.status === "pending" && info.status !== "dissolved" ? (
                <div className="mt-2 flex gap-2">
                  <Button
                    type="button"
                    disabled={pending || info.status === "full"}
                    onClick={() => run(`/v1/posts/${post.id}/practice/applications/${application.id}/accept`)}
                  >
                    承認
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={pending}
                    onClick={() =>
                      run(
                        `/v1/posts/${post.id}/practice/applications/${application.id}/decline`,
                        "この参加希望を見送りますか？",
                      )
                    }
                  >
                    見送り
                  </Button>
                </div>
              ) : null}
              {application.status === "accepted" && info.status === "dissolved" ? (
                application.rated ? (
                  <p className="mt-2 text-sm">評価済み</p>
                ) : (
                  <RatingForm disabled={pending} onSubmit={(score, comment) => rate(application.id, score, comment)} />
                )
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

function PracticeFacts({ post }: { post: Post }) {
  const info = post.practice;
  if (!info) return null;
  const rows = [
    { label: "練習場所", value: info.venue },
    { label: "練習日時", value: formatDateTime(info.starts_at) },
    { label: "種目", value: info.event_label },
    { label: "募集人数", value: `${info.accepted_count}/${info.capacity}人` },
    { label: "状態", value: practiceStatusLabel(info.status) },
  ];
  return (
    <div className="space-y-4">
      <dl className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
        {rows.map((row) => (
          <div key={row.label}>
            <dt className="text-sm text-slate-200">{row.label}</dt>
            <dd className="mt-1 whitespace-pre-wrap">{row.value}</dd>
          </div>
        ))}
      </dl>
      <PracticeReputationSummary reputation={info.reputation} heading="これまでの評価" />
    </div>
  );
}

function RecruitmentFacts({ post }: { post: Post }) {
  const info = post.recruitment;
  if (!info) return null;
  const place = info.prefectures.map((code) => prefectureName(code)).join("、");
  const location = [place, info.venue].filter(Boolean).join(" ");
  const rows = [
    { label: "活動場所", value: location },
    { label: "活動時間", value: info.schedule },
    { label: "男女比", value: info.gender_ratio },
    { label: "平均年齢", value: info.average_age },
    { label: "募集年齢", value: info.recruit_age },
  ].filter((row) => row.value);

  return (
    <div className="space-y-4">
      {info.photos.length > 0 ? (
        <div className={info.photos.length > 1 ? "grid gap-2 md:grid-cols-2" : "grid gap-2"}>
          {info.photos.map((photo) => (
            <ZoomableImage
              key={photo.public_id}
              src={photo.url}
              alt=""
              fill
              sizes="(max-width: 768px) 100vw, 50vw"
              wrapperClassName="relative aspect-[4/3] w-full overflow-hidden rounded-xl bg-black/25"
              imageClassName="object-cover"
            />
          ))}
        </div>
      ) : null}
      <dl className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
        {rows.map((row) => (
          <div key={row.label}>
            <dt className="text-sm text-slate-200">{row.label}</dt>
            <dd className="mt-1 whitespace-pre-wrap">{row.value}</dd>
          </div>
        ))}
        {info.links.length > 0 ? (
          <div>
            <dt className="text-sm text-slate-200">SNS</dt>
            <dd className="mt-1 space-y-1">
              {info.links.map((link) => (
                <a key={link} href={link} target="_blank" rel="noreferrer" className="block break-all text-blue-400 transition active:scale-95">
                  {link}
                </a>
              ))}
            </dd>
          </div>
        ) : null}
      </dl>
    </div>
  );
}
