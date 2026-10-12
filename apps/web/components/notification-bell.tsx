"use client";

import { apiFetch } from "@/lib/api";
import { formatRelativeTime } from "@/lib/format";
import type { AppNotification, Meta, NotificationKind } from "@/lib/types";
import { Bell } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { useAuth } from "./auth-provider";

const labels: Record<NotificationKind, string> = {
  recruitment_post: "新しい募集",
  comment: "コメントがありました",
  practice_application: "参加申請がありました",
  practice_accepted: "参加が承認されました",
  practice_rate: "解散しました。評価しあってください",
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
    async (list: boolean) => {
      if (!token) return;
      if (list) setLoading(true);
      try {
        const data = await apiFetch<{ notifications: AppNotification[]; unread_count: number; meta: Meta }>(
          `/v1/me/notifications?per_page=${list ? 20 : 1}`,
          { token },
        );
        setUnread(data.unread_count ?? 0);
        if (list) {
          setItems(data.notifications ?? []);
          setError("");
        }
      } catch {
        if (list) setError("通知を読み込めませんでした");
      } finally {
        if (list) setLoading(false);
      }
    },
    [token],
  );

  useEffect(() => {
    if (!token) return;
    const refresh = () => {
      if (document.visibilityState === "visible") void load(open);
    };
    void load(open);
    document.addEventListener("visibilitychange", refresh);
    return () => document.removeEventListener("visibilitychange", refresh);
  }, [token, open, load]);

  useEffect(() => {
    if (!open) return;
    function onPointer(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

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
        onClick={() => setOpen((value) => !value)}
        className="relative inline-flex size-11 items-center justify-center rounded-md text-zinc-700 hover:bg-zinc-100"
      >
        <Bell className="size-5" strokeWidth={2} aria-hidden />
        {unread > 0 ? (
          <span className="absolute top-1 right-1 inline-flex min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] leading-4 font-semibold text-white">
            {badgeText(unread)}
          </span>
        ) : null}
      </button>
      {open ? (
        <div
          id={panelID}
          className="absolute right-0 z-50 mt-1 w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-zinc-200 bg-white shadow-lg"
        >
          <div className="flex items-center justify-between border-b border-zinc-200 px-3 py-2">
            <p className="text-sm font-semibold">通知</p>
            {unread > 0 ? (
              <button type="button" onClick={() => void markAll()} className="text-xs text-blue-700 hover:underline">
                すべて既読
              </button>
            ) : null}
          </div>
          {loading && items.length === 0 ? <p className="px-3 py-6 text-center text-sm text-zinc-500">読み込み中</p> : null}
          {error ? <p className="px-3 py-3 text-sm text-red-700">{error}</p> : null}
          {!loading && !error && items.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-zinc-500">通知はありません</p>
          ) : null}
          {items.length > 0 ? (
            <ul className="max-h-96 overflow-y-auto">
              {items.map((item) => (
                <li key={item.id} className={item.read ? "" : "bg-blue-50"}>
                  <Link
                    href={`/posts/${item.post_id}`}
                    onClick={() => {
                      setOpen(false);
                      if (!item.read) void markRead(item.id);
                    }}
                    className="block px-3 py-3 hover:bg-zinc-50"
                  >
                    <p className="text-sm">
                      {labelFor(item.kind)}: {item.post_title}
                    </p>
                    <p className="mt-1 text-xs text-zinc-500">{formatRelativeTime(item.created_at)}</p>
                  </Link>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
