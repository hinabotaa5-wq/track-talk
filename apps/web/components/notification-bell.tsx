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
};

function labelFor(kind: string) {
  return kind in labels ? labels[kind as NotificationKind] : "通知";
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
      if (document.visibilityState === "visible") load(open);
    };
    load(open);
    document.addEventListener("visibilitychange", refresh);
    return () => document.removeEventListener("visibilitychange", refresh);
  }, [token, open, load]);

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
          if (!open) setLoading(true);
          setOpen(!open);
        }}
        className="relative inline-flex size-11 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 active:scale-95"
      >
        <Bell className="size-5" strokeWidth={2} aria-hidden />
        {unread > 0 ? (
          <span className="absolute top-1 right-1 inline-flex min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] leading-4 font-semibold text-white">
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
              <button type="button" onClick={() => void markAll()} className="text-xs text-blue-400 transition hover:underline active:scale-95">
                すべて既読
              </button>
            ) : null}
          </div>
          {loading && items.length === 0 ? <p className="px-3 py-6 text-center text-sm text-slate-200">読み込み中</p> : null}
          {error ? <p className="px-3 py-3 text-sm text-red-400">{error}</p> : null}
          {!loading && !error && items.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-slate-200">通知はありません</p>
          ) : null}
          {items.length > 0 ? (
            <ul className="max-h-96 overflow-y-auto">
              {items.map((item) => (
                <li key={item.id} className={item.read ? "" : "bg-blue-950/40"}>
                  <Link
                    href={`/posts/${item.post_id}`}
                    onClick={() => {
                      setOpen(false);
                      if (!item.read) void markRead(item.id);
                    }}
                    className="block px-3 py-3 transition hover:bg-white/16 active:scale-95"
                  >
                    <p className="text-sm text-slate-100">
                      {labelFor(item.kind)}: {item.post_title}
                    </p>
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
