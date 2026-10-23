"use client";

import { apiFetch } from "@/lib/api";
import { formatRelativeTime } from "@/lib/format";
import type { AppNotification, Meta, NotificationKind } from "@/lib/types";
import { Bell } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { AnchoredPanel } from "./anchored-panel";
import { useAuth } from "./auth-provider";

const labels: Record<NotificationKind, string> = {
  recruitment_post: "新しい募集",
  comment: "コメントがありました",
  comment_reply: "あなたのコメントに返信がありました",
  practice_application: "参加申請がありました",
  practice_accepted: "参加が承認されました",
  practice_rate: "解散しました。評価しあってください",
  injury_update: "復帰の経過が更新されました",
  review_offer: "フォーム添削の依頼が届きました",
  review_expired: "フォーム添削の依頼が期限を過ぎました",
  review_returned: "フォーム添削の返答が届きました",
  review_clarify: "添削の補足を頼まれました",
  review_clarified: "添削の補足が届きました",
  review_helpful_ask: "アドバイスは役に立ちましたか",
  message: "お知らせ",
};

// 通知を開いたときの行き先。添削の通知は投稿ではなく、コーチ画面か依頼の画面へ。
function hrefFor(item: AppNotification) {
  // サイト内のパスだけを開く。
  if (item.url?.startsWith("/") && !item.url.startsWith("//")) return item.url;
  if (item.kind === "review_offer") return "/coach";
  if (item.kind === "review_clarify" && item.review_request_id) return `/coach/requests/${item.review_request_id}`;
  if (item.review_request_id) return `/coaching/requests/${item.review_request_id}`;
  return `/posts/${item.post_id}`;
}

function labelFor(item: AppNotification) {
  if (item.title) return item.title;
  return item.kind in labels ? labels[item.kind] : "通知";
}

function badgeText(count: number) {
  if (count > 99) return "99+";
  return String(count);
}

export function NotificationBell() {
  const { token } = useAuth();
  const panelID = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [unread, setUnread] = useState(0);
  const [items, setItems] = useState<AppNotification[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  // タブに戻ったときの読み直しは、effect を作った時点ではなく、いま開いているかで決める。
  const openRef = useRef(false);
  useEffect(() => {
    openRef.current = open;
  }, [open]);

  const load = useCallback(
    (list: boolean) => {
      if (!token) return;
      apiFetch<{ notifications: AppNotification[]; unread_count: number; meta: Meta }>(
        `/v1/me/notifications?per_page=${list ? 20 : 1}`,
        { token },
      )
        .then((data) => {
          setUnread(data.unread_count ?? 0);
          if (list) {
            setItems(data.notifications ?? []);
            setError("");
          }
        })
        .catch(() => {
          if (list) setError("通知を読み込めませんでした");
        })
        .finally(() => {
          if (list) setLoading(false);
        });
    },
    [token],
  );

  useEffect(() => {
    if (!token) return;
    const refresh = () => {
      if (document.visibilityState === "visible") load(openRef.current);
    };
    // 初回は即フェッチ、以降はタブ復帰時のみ
    load(openRef.current);
    document.addEventListener("visibilitychange", refresh);
    return () => document.removeEventListener("visibilitychange", refresh);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  async function markRead(id: string) {
    if (!token) return;
    try {
      await apiFetch(`/v1/me/notifications/${id}/read`, { method: "POST", token });
      setItems((current) => current.map((item) => (item.id === id ? { ...item, read: true } : item)));
      setUnread((count) => Math.max(0, count - 1));
    } catch {
      setError("既読にできませんでした");
    }
  }

  async function markAll() {
    if (!token) return;
    try {
      await apiFetch("/v1/me/notifications/read", { method: "POST", token });
      setItems((current) => current.map((item) => ({ ...item, read: true })));
      setUnread(0);
    } catch {
      setError("既読にできませんでした");
    }
  }

  const label = unread > 0 ? `通知、未読${badgeText(unread)}件` : "通知";

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-expanded={open}
        aria-controls={panelID}
        onClick={() => {
          const willOpen = !open;
          if (willOpen && items.length === 0) setLoading(true);
          if (willOpen) load(true);
          setOpen(willOpen);
        }}
        className="relative inline-flex size-11 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 active:scale-[0.98]"
      >
        <Bell className="size-5" strokeWidth={2} aria-hidden />
        {unread > 0 ? (
          <span className="absolute top-1 right-1 inline-flex min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-xs leading-4 font-semibold text-white">
            {badgeText(unread)}
          </span>
        ) : null}
      </button>
      <AnchoredPanel
        open={open}
        anchorRef={rootRef}
        panelID={panelID}
        onClose={() => setOpen(false)}
        className="w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-white/20 bg-[#16243a] shadow-xl"
      >
          <div className="flex items-center justify-between border-b border-white/20 px-3 py-2">
            <p className="text-sm font-semibold text-slate-100">通知</p>
            {unread > 0 ? (
              <button type="button" onClick={() => void markAll()} className="text-xs text-blue-400 transition hover:underline active:scale-[0.98]">
                すべて既読
              </button>
            ) : null}
          </div>
          {loading && items.length === 0 ? <p className="px-3 py-6 text-center text-sm text-slate-200">読み込み中</p> : null}
          {error && items.length === 0 ? <p className="px-3 py-3 text-sm text-red-400">{error}</p> : null}
          {!loading && !error && items.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-slate-200">通知はありません</p>
          ) : null}
          {items.length > 0 ? (
            <ul className="max-h-96 overflow-y-auto">
              {items.map((item) => (
                <li key={item.id} className={item.read ? "border-l-4 border-transparent bg-white/5" : "border-l-4 border-sky-300 bg-sky-400/20"}>
                  <Link
                    href={hrefFor(item)}
                    onClick={() => {
                      setOpen(false);
                      if (!item.read) void markRead(item.id);
                    }}
                    className="block px-3 py-3 transition hover:bg-white/16 active:scale-[0.98]"
                  >
                    <p className={`text-sm ${item.read ? "font-normal text-slate-300" : "font-bold text-white"}`}>
                      <span className={`mr-2 inline-block rounded-full px-2 py-0.5 text-xs ${item.read ? "bg-white/10 text-slate-400" : "bg-sky-300 text-slate-950"}`}>{item.read ? "既読" : "未読"}</span>
                      {labelFor(item)}
                      {item.post_title ? `: ${item.post_title}` : ""}
                    </p>
                    {item.body ? <p className="mt-1 text-xs text-slate-300">{item.body}</p> : null}
                    <p className="mt-1 text-xs text-slate-200">{formatRelativeTime(item.created_at)}</p>
                  </Link>
                </li>
              ))}
            </ul>
          ) : null}
      </AnchoredPanel>
    </div>
  );
}
