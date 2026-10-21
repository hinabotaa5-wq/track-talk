"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import type { Comment, Meta, Post } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import { feedTitle, hasFeedTitle } from "@/components/feed-card";
import { MeetThreadNowRunning } from "@/components/meet-now-running";
import { DetailLoadingFrame } from "@/components/loading-frame";
import { PracticeReputationSummary, practiceAttendanceLine } from "@/components/practice-reputation";
import { prefecturePageHref } from "@/lib/prefecture-page";
import { prefectureName } from "@/lib/prefectures";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { canCommentOnPost } from "@/lib/practice-comments";
import { buildCommentThreads, replyLabel, type CommentThread } from "@/lib/comment-thread";
import { commentSchema } from "@/lib/validators";
import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ZoomableImage } from "@/components/zoomable-image";
import { ZoomableVideo } from "@/components/zoomable-video";
import { InstagramEmbed } from "@/components/instagram-embed";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { YouTubePlayer } from "@/components/youtube-player";
import { LikeButton, ReportButton, SaveButton } from "@/components/action-button";
import { InjuryTypicalRecovery } from "@/components/injury-typical-recovery";
import { clearHomeFeeds, clearSavedFeeds } from "@/lib/home-feed-cache";

export function PostDetail({
  id,
  returnTo,
  initial,
}: {
  id: string;
  returnTo?: "me";
  // サーバーでログインなしに読めた投稿。最初からこれを描き、ログイン中ならいいね状態などを読み直す。
  initial?: { post: Post; comments: Comment[] };
}) {
  const { session, token } = useAuth();
  const router = useRouter();
  const [post, setPost] = useState<Post | null>(initial?.post ?? null);
  const [comments, setComments] = useState<Comment[]>(initial?.comments ?? []);
  const [error, setError] = useState("");
  const [commentError, setCommentError] = useState("");
  const [loading, setLoading] = useState(!initial);
  const form = useForm({ resolver: zodResolver(commentSchema), defaultValues: { body: "" } });
  const action = useActionLock();
  const confirmDialog = useConfirm();
  const commentLock = useActionLock();
  const [replyTo, setReplyTo] = useState<Comment | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(null);
  // コメントのカードをタップして開いたスレッド(null なら二列のカード一覧)。
  // URL の ?room= に持たせるので、ヘッダーの戻る(＜)やブラウザの戻るで一覧に戻れる。
  const searchParams = useSearchParams();
  const openThreadId = searchParams.get("room");
  const [reportedPost, setReportedPost] = useState(false);
  const [reportedComments, setReportedComments] = useState<Set<string>>(new Set());

  // load は何度も重なって呼ばれる(ログイン状態の確定、タブへの復帰、いいねのあとなど)。
  // 後から始めたものの結果だけを使い、先に始めた古い応答で上書きしない。
  const loadSeq = useRef(0);
  async function load() {
    const seq = ++loadSeq.current;
    const stale = () => seq !== loadSeq.current;
    try {
      const [p, c] = await Promise.all([apiFetch<Post>(`/v1/posts/${id}`, { token }), loadAllComments(id, token)]);
      if (stale()) return;
      setPost(p);
      setComments(c);
      if (token) {
        const mine = await apiFetch<{ post: boolean; comment_ids: string[] }>(`/v1/posts/${id}/my-reports`, { token }).catch(() => null);
        if (stale()) return;
        if (mine) {
          setReportedPost(mine.post);
          setReportedComments(new Set(mine.comment_ids));
        }
      }
      setError("");
    } catch (e) {
      if (stale()) return;
      if (e instanceof ApiRequestError && e.status === 404) setError("投稿が見つかりません");
      else if (e instanceof ApiRequestError && (e.status === 401 || e.code === "not_joined")) {
        // ケガの投稿は、同じケガのグループに参加した人だけが読める。
        setError("この投稿は、同じケガのグループに参加すると読めます");
      } else setError("読み込みに失敗しました");
    } finally {
      if (!stale()) setLoading(false);
    }
  }

  useEffect(() => () => document.body.removeAttribute("data-comment-typing"), []);

  // コメントルームの中では、返信先を選ばなければそのスレッドの最初のコメントへの返信になる。
  const roomTop = openThreadId ? (comments.find((c) => c.id === openThreadId && !c.parent_id) ?? null) : null;
  // 選んだ返信先は、開いているルームのものだけ使う(＜で一覧に戻ったら「返信中」を出さない)。
  const roomReplyTo = roomTop && replyTo && (replyTo.id === roomTop.id || replyTo.parent_id === roomTop.id) ? replyTo : null;
  const replyTarget = roomTop ? (roomReplyTo ?? roomTop) : null;

  // 長押しで開く操作メニューの対象。
  const [menuComment, setMenuComment] = useState<Comment | null>(null);
  const longPress = useRef<{ timer: number; x: number; y: number } | null>(null);
  const chatEnd = useRef<HTMLDivElement>(null);
  const openThreadReplyCount = comments.filter((c) => c.parent_id === openThreadId).length;
  useEffect(() => {
    // チャットのように、スレッドを開いたときと返信が増えたときは一番下を見せる。
    if (openThreadId) chatEnd.current?.scrollIntoView({ block: "end" });
  }, [openThreadId, openThreadReplyCount]);

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
    } catch (e) {
      alert(e instanceof ApiRequestError ? e.message : "いいねできませんでした");
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
    } catch (e) {
      alert(e instanceof ApiRequestError ? e.message : "保存できませんでした");
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
    router.push(returnTo === "me" ? "/me/posts" : current.kind === "injury" ? "/injury" : "/");
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
    if (!(await confirmDialog(reportConfirmOptions()))) return;
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
        body: JSON.stringify(replyTarget ? { ...values, reply_to_id: replyTarget.id } : values),
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

  function openThread(c: Comment) {
    const params = new URLSearchParams(searchParams.toString());
    params.set("room", c.id);
    window.history.pushState(null, "", `?${params.toString()}`);
    setReplyTo(null);
  }

  function renderCommentCard(thread: CommentThread & { top: Comment }) {
    const c = thread.top;
    return (
      <button
        key={c.id}
        type="button"
        onClick={() => openThread(c)}
        className="flex min-h-28 flex-col rounded-xl border border-white/20 glass bg-white/10 p-3 text-left text-slate-200 transition hover:bg-white/16 active:scale-[0.98]"
      >
        <span className="mb-1 truncate text-xs font-semibold text-slate-100">{c.author_name ?? "匿名"}</span>
        <span className="line-clamp-4 whitespace-pre-wrap break-words text-sm">{c.body}</span>
        <span className="mt-auto flex items-center justify-between gap-2 pt-2 text-xs text-slate-300">
          <span className={thread.replies.length > 0 ? "font-semibold text-blue-200" : ""}>💬 {thread.replies.length}</span>
          <span className="truncate">{formatRelativeTime(c.created_at)}</span>
        </span>
      </button>
    );
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
    } catch (e) {
      alert(e instanceof ApiRequestError ? e.message : "編集できませんでした");
    } finally {
      action.release();
    }
  }

  async function deleteComment(cid: string) {
    if (!confirm("このコメントを削除しますか？")) return;
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

  function startPress(e: React.PointerEvent, c: Comment) {
    cancelPress();
    longPress.current = {
      x: e.clientX,
      y: e.clientY,
      timer: window.setTimeout(() => {
        longPress.current = null;
        setMenuComment(c);
      }, 450),
    };
  }

  function movePress(e: React.PointerEvent) {
    const p = longPress.current;
    if (p && Math.hypot(e.clientX - p.x, e.clientY - p.y) > 10) cancelPress();
  }

  function cancelPress() {
    if (longPress.current) window.clearTimeout(longPress.current.timer);
    longPress.current = null;
  }

  // 返信元を吹き出しの中に小さく引用する。本文は手元のコメントから引く。
  function renderQuote(c: Comment) {
    // 最初のコメント(スレッドの親)宛ての返信には、引用を付けない。
    if (!c.reply_to_id || c.reply_to_id === c.parent_id) return null;
    const target = comments.find((x) => x.id === c.reply_to_id);
    return (
      <button
        type="button"
        className="mb-1.5 block w-full rounded-lg border-l-2 border-white/50 bg-black/20 px-2 py-1 text-left text-xs disabled:opacity-70"
        disabled={c.reply_to_removed}
        onClick={(e) => {
          e.stopPropagation();
          jumpToComment(c.reply_to_id as string);
        }}
        onPointerDown={(e) => e.stopPropagation()}
      >
        <span className="block truncate font-semibold">{replyLabel(c)}</span>
        <span className="line-clamp-2 break-words opacity-80">{target && !c.reply_to_removed ? target.body : "削除されたコメント"}</span>
      </button>
    );
  }

  function renderBubble(c: Comment) {
    return (
      <div key={c.id} id={`comment-${c.id}`} className={`flex flex-col ${c.mine ? "items-end" : "items-start"}`}>
        {c.mine ? null : <span className="mb-0.5 ml-3 text-xs font-semibold text-slate-200">{c.author_name ?? "匿名"}</span>}
        <div className={`flex max-w-[85%] items-end gap-1.5 ${c.mine ? "flex-row-reverse" : ""}`}>
          <div
            onPointerDown={(e) => startPress(e, c)}
            onPointerMove={movePress}
            onPointerUp={cancelPress}
            onPointerCancel={cancelPress}
            onPointerLeave={cancelPress}
            onContextMenu={(e) => {
              e.preventDefault();
              cancelPress();
              setMenuComment(c);
            }}
            className={`select-none rounded-3xl px-4 py-2.5 text-sm text-white transition-colors [-webkit-touch-callout:none] ${
              c.mine ? "rounded-br-md bg-blue-500/70" : "rounded-bl-md border border-white/20 bg-white/15"
            } ${highlightId === c.id ? "ring-2 ring-blue-300" : ""}`}
          >
            {renderQuote(c)}
            <p className="whitespace-pre-wrap break-words">{c.body}</p>
          </div>
          <span className="shrink-0 pb-0.5 text-xs text-slate-300">{formatRelativeTime(c.created_at)}</span>
        </div>
      </div>
    );
  }

  function renderMenu(c: Comment) {
    const item = "w-full px-4 py-3.5 text-center text-base font-semibold transition active:bg-white/10 disabled:opacity-50";
    const close = () => setMenuComment(null);
    return (
      <div className="fixed inset-0 z-40 flex items-end bg-black/50" onClick={close}>
        <div
          className="mx-auto w-full max-w-md divide-y divide-white/15 overflow-hidden rounded-t-3xl border border-white/20 bg-slate-900/95 pb-[env(safe-area-inset-bottom)] text-slate-100"
          onClick={(e) => e.stopPropagation()}
        >
          <p className="truncate px-4 py-3 text-center text-xs text-slate-300">{c.body}</p>
          {allowComment ? (
            <button
              type="button"
              className={item}
              onClick={() => {
                close();
                startReply(c);
              }}
            >
              返信
            </button>
          ) : null}
          {c.mine ? (
            <>
              <button
                type="button"
                className={`${item} text-blue-300`}
                disabled={action.pending}
                onClick={() => {
                  close();
                  void editComment(c);
                }}
              >
                編集
              </button>
              <button
                type="button"
                className={`${item} text-red-400`}
                disabled={action.pending}
                onClick={() => {
                  close();
                  void deleteComment(c.id);
                }}
              >
                削除
              </button>
            </>
          ) : (
            <button
              type="button"
              className={item}
              disabled={action.pending || reportedComments.has(c.id)}
              onClick={() => {
                close();
                void report("comment", c.id);
              }}
            >
              {reportedComments.has(c.id) ? "通報済み" : "通報"}
            </button>
          )}
          <button type="button" className={`${item} text-slate-300`} onClick={close}>
            キャンセル
          </button>
        </div>
      </div>
    );
  }

  const threads = buildCommentThreads(comments);
  const openedThread = threads.find((t) => t.top?.id === openThreadId);
  const activeThread = openedThread?.top ? { top: openedThread.top, replies: openedThread.replies } : null;

  return (
    <article
      className={`${allowComment ? "pb-[calc(12.5rem+env(safe-area-inset-bottom))] md:pb-36 lg:pb-0" : ""} ${activeThread ? "mx-auto max-w-2xl" : "lg:grid lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start lg:gap-8"}`}
    >
      {/* コメントルーム(スレッド)の間は、投稿の動画・いいね・保存などを隠してチャットに集中させる。 */}
      {activeThread ? null : (
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
      ) : post.kind === "instagram" && post.instagram_type && post.instagram_shortcode ? (
        <InstagramEmbed type={post.instagram_type} shortcode={post.instagram_shortcode} title={feedTitle(post.title)} />
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
      {post.kind === "practice" || ((post.kind === "youtube" || post.kind === "instagram") && !hasFeedTitle(post.title)) ? null : (
        <h1 className="text-4xl font-bold [overflow-wrap:anywhere]">{feedTitle(post.title)}</h1>
      )}
      {post.kind === "meet" ? <MeetThreadNowRunning postId={post.id} /> : null}
      {post.source_title ? (
        <p className="text-xs text-slate-400">元動画: {post.source_title}</p>
      ) : null}
      <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-slate-200">
        {post.author_name ? <span>{post.author_name}</span> : null}
        {post.author_name && post.author_post_master ? (
          <span
            title="いいねがついた投稿が30件以上ある人"
            className="rounded-full border border-amber-300/40 bg-amber-400/15 px-2 py-0.5 text-xs font-semibold text-amber-200"
          >
            投稿マスター
          </span>
        ) : null}
        <span className="ml-auto text-xs text-slate-300">{formatDateTime(post.created_at)}</span>
      </p>
      {(post.kind === "recruitment" || post.kind === "practice") && post.body ? (
        <div>
          <h2 className="mb-2 text-2xl font-semibold text-slate-100">詳細</h2>
          <div className="whitespace-pre-wrap [overflow-wrap:anywhere] rounded-xl border border-white/20 glass bg-white/10 p-4 text-lg leading-8 text-slate-200">{post.body}</div>
        </div>
      ) : post.body ? (
        <div className="whitespace-pre-wrap [overflow-wrap:anywhere] rounded-xl border border-white/20 glass bg-white/10 p-4 text-lg leading-8 text-slate-200">{post.body}</div>
      ) : null}
      {post.kind === "injury" && post.injury ? (
        <InjuryTypicalRecovery bodyPart={post.injury.body_part} bodyPartLabel={post.injury.body_part_label} />
      ) : null}
      <div className="flex flex-wrap items-center gap-2.5 pt-2">
        {post.kind === "meet" ? null : (
          <>
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
          </>
        )}
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
        {post.kind === "meet" ? null : (
          <ReportButton reported={reportedPost} disabled={action.pending} onClick={() => report("post", post.id)} />
        )}
      </div>
      </div>
      )}
      <section className={`space-y-3 ${activeThread ? "" : "mt-6 lg:sticky lg:top-4 lg:mt-0"}`}>
        {activeThread ? (
          <h2 className="truncate text-xl font-semibold">{feedTitle(post.title)}</h2>
        ) : (
          <h2 className="text-xl font-semibold">コメント</h2>
        )}
        {comments.length === 0 ? <p className="text-sm text-slate-200">まだコメントはありません。</p> : null}
        {activeThread ? (
          <div className="space-y-3">
            <p className="text-center text-xs text-slate-300">吹き出しを長押しで、メンション・編集・削除・通報ができます</p>
            <div className="space-y-3">
              {renderBubble(activeThread.top)}
              {activeThread.replies.map((r) => renderBubble(r))}
              <div ref={chatEnd} className="scroll-mb-44" />
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-2.5">
            {threads.map((thread) =>
              thread.top ? (
                renderCommentCard({ ...thread, top: thread.top })
              ) : (
                <p key={thread.replies[0].id} className="rounded-xl border border-dashed border-white/20 p-3 text-xs text-slate-300">
                  削除されたコメント（返信{thread.replies.length}件）
                </p>
              ),
            )}
          </div>
        )}
        {allowComment ? (
          <form
            onSubmit={(event) => void form.handleSubmit(onComment)(event)}
            onFocus={() => document.body.setAttribute("data-comment-typing", "")}
            onBlur={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                document.body.removeAttribute("data-comment-typing");
              }
            }}
            className="glass fixed inset-x-0 bottom-[calc(3.5rem+env(safe-area-inset-bottom))] [body[data-comment-typing]_&]:bottom-0 z-20 border-t border-white/20 bg-white/10 p-3 md:bottom-0 md:pb-[max(0.75rem,env(safe-area-inset-bottom))] lg:static lg:bottom-auto lg:rounded-xl lg:border lg:p-3 lg:pb-3"
          >
            {roomReplyTo ? (
              <div className="mx-auto mb-2 flex w-full max-w-5xl items-center gap-2 rounded-lg bg-blue-400/20 px-3 py-1.5 text-xs text-blue-100 lg:mx-0 lg:max-w-none">
                <span className="min-w-0 flex-1 truncate">
                  <span className="font-semibold">@{replyTargetName(roomReplyTo)}</span> に返信中: {roomReplyTo.body}
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
      {menuComment ? renderMenu(menuComment) : null}
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
  allowNoShow = false,
  onSubmit,
}: {
  disabled: boolean;
  allowNoShow?: boolean;
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
          {allowNoShow ? <option value="no_show">ドタキャン（当日来なかった）</option> : null}
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
      {!mine && info.ratable && info.my_application === "accepted" && info.my_application_id ? (
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
              <p className="mt-1 text-sm text-slate-200">{practiceAttendanceLine(application.attendance)}</p>
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
              {application.status === "accepted" && info.ratable ? (
                application.rated ? (
                  <p className="mt-2 text-sm">評価済み</p>
                ) : (
                  <RatingForm
                    disabled={pending}
                    allowNoShow
                    onSubmit={(score, comment) => rate(application.id, score, comment)}
                  />
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
      <PracticeReputationSummary
        reputation={info.reputation}
        heading="これまでの評価"
        reportNext={post.mine ? undefined : `/posts/${post.id}`}
      />
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
      {info.prefectures.length > 0 ? (
        <p className="flex flex-wrap gap-x-3 gap-y-1 text-sm">
          {info.prefectures.map((code) => (
            <Link key={code} href={prefecturePageHref(code)} className="text-blue-300 underline">
              {prefectureName(code)}の大会
            </Link>
          ))}
        </p>
      ) : null}
    </div>
  );
}

// コメントは1回に50件までしか返らないので、全件をページごとに読む(新しいコメントは後ろのページにある)。
async function loadAllComments(id: string, token: string | null) {
  const all: Comment[] = [];
  for (let page = 1; page <= 40; page++) {
    const res = await apiFetch<{ comments: Comment[]; meta: Meta }>(`/v1/posts/${id}/comments?per_page=50&page=${page}`, { token });
    const rows = res.comments ?? [];
    all.push(...rows);
    if (rows.length === 0 || all.length >= res.meta.total) break;
  }
  return all;
}
