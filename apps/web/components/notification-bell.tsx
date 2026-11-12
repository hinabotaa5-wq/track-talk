"use client";

import { PageLoading } from "@/components/page-loading";

import { apiFetch } from "@/lib/api";
import { applyAppBadge, clearAppBadge } from "@/lib/app-badge";
import { formatRelativeTime } from "@/lib/format";
import { t, type I18nKey, type Locale } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import type { AppNotification, Meta, NotificationKind } from "@/lib/types";
import { Bell } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { AnchoredPanel } from "./anchored-panel";
import { useAuth } from "./auth-provider";

// ベルの種類ごとの表示名。API が個別の表題を付けた通知（title あり）はそちらを出す。
// 未訳の種類は日本語にフォールバックする（本体 #521）。
const labelKeys: Record<NotificationKind, I18nKey> = {
  recruitment_post: "notify.recruitment_post",
  comment: "notify.comment",
  comment_reply: "notify.comment_reply",
  practice_application: "notify.practice_application",
  practice_accepted: "notify.practice_accepted",
  practice_rate: "notify.practice_rate",
  injury_update: "notify.injury_update",
  review_offer: "notify.review_offer",
  review_offer_reminder: "notify.review_offer_reminder",
  review_accepted: "notify.review_accepted",
  review_nomination_ended: "notify.review_nomination_ended",
  review_expired: "notify.review_expired",
  review_returned: "notify.review_returned",
  review_clarify: "notify.review_clarify",
  review_clarified: "notify.review_clarified",
  review_info_request: "notify.review_info_request",
  review_resubmitted: "notify.review_resubmitted",
  review_resubmitted_after_accept: "notify.review_resubmitted_after_accept",
  review_withdrawn: "notify.review_withdrawn",
  review_helpful_ask: "notify.review_helpful_ask",
  message: "notify.message",
};

// 通知を開いたときの行き先。添削の通知は投稿ではなく、コーチ画面か依頼の画面へ。
function hrefFor(item: AppNotification) {
  // サイト内のパスだけを開く。
  if (item.url?.startsWith("/") && !item.url.startsWith("//")) return item.url;
  if (item.kind === "review_offer" || item.kind === "review_offer_reminder" ||
      item.kind === "review_resubmitted" || item.kind === "review_nomination_ended") return "/coach";
  if (item.kind === "review_clarify" && item.review_request_id) return `/coach/requests/${item.review_request_id}`;
  if (item.review_request_id) return `/coaching/requests/${item.review_request_id}`;
  return `/posts/${item.post_id}`;
}

function labelFor(item: AppNotification, locale: Locale) {
  if (item.title) return item.title;
  const key = labelKeys[item.kind];
  return key ? t(locale, key) : t(locale, "notify.title");
}

function badgeText(count: number) {
  if (count > 99) return "99+";
  return String(count);
}

export function NotificationBell() {
  const { token } = useAuth();
  const locale = useLocale();
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
          // バッジはベルの未読数と同じ正本（unread_count）から写す。ローカルで増減を数えない。
          void applyAppBadge(data.unread_count);
          if (list) {
            setItems(data.notifications ?? []);
            setError("");
          }
        })
        .catch(() => {
          if (list) setError(t(locale, "notify.loadError"));
        })
        .finally(() => {
          if (list) setLoading(false);
        });
    },
    [token, locale],
  );

  useEffect(() => {
    // 未ログイン状態ではバッジを付けない。ログアウト時は消す。
    if (!token) {
      void clearAppBadge();
      return;
    }
    const refresh = () => {
      if (document.visibilityState === "visible") load(openRef.current);
    };
    // 初回は即フェッチ、以降はフォアグラウンド復帰時
    load(openRef.current);
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("focus", refresh);
    return () => {
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("focus", refresh);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  async function markRead(id: string) {
    if (!token) return;
    try {
      await apiFetch(`/v1/me/notifications/${id}/read`, { method: "POST", token });
      setItems((current) => current.map((item) => (item.id === id ? { ...item, read: true } : item)));
      setUnread((count) => Math.max(0, count - 1));
      // バッジは正本の最新数で付け直す。
      load(false);
    } catch {
      setError(t(locale, "notify.readError"));
    }
  }

  async function markAll() {
    if (!token) return;
    try {
      await apiFetch("/v1/me/notifications/read", { method: "POST", token });
      setItems((current) => current.map((item) => ({ ...item, read: true })));
      setUnread(0);
      // バッジは正本の最新数で付け直す。
      load(false);
    } catch {
      setError(t(locale, "notify.readError"));
    }
  }

  const label = unread > 0 ? t(locale, "notify.unread", { count: badgeText(unread) }) : t(locale, "notify.title");

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
        className={`relative inline-flex size-11 items-center justify-center rounded-full transition active:scale-[0.98] ${open ? "bg-white/20 text-white" : "text-slate-300 hover:bg-white/16"}`}
      >
        <Bell className="size-5" strokeWidth={2} aria-hidden />
        {unread > 0 ? (
          <span className="absolute top-1 right-1 inline-flex min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-xs leading-4 font-semibold text-pure-white">
            {badgeText(unread)}
          </span>
        ) : null}
      </button>
      <AnchoredPanel
        open={open}
        anchorRef={rootRef}
        panelID={panelID}
        onClose={() => setOpen(false)}
        className="w-80 max-w-[calc(100vw-2rem)] overflow-hidden rounded-xl border border-white/20 bg-surface shadow-xl"
      >
          <div className="flex items-center justify-between border-b border-white/20 px-3 py-2">
            <p className="text-sm font-semibold text-slate-100">{t(locale, "notify.title")}</p>
            {unread > 0 ? (
              <button type="button" onClick={() => void markAll()} className="text-xs text-blue-400 transition hover:underline active:scale-[0.98]">
                {t(locale, "notify.markAll")}
              </button>
            ) : null}
          </div>
          {loading && items.length === 0 ? <PageLoading label={t(locale, "common.loading")} /> : null}
          {error && items.length === 0 ? <p className="px-3 py-3 text-sm text-red-400">{error}</p> : null}
          {!loading && !error && items.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-slate-200">{t(locale, "notify.empty")}</p>
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
                      <span className={`mr-2 inline-block rounded-full px-2 py-0.5 text-xs ${item.read ? "bg-white/10 text-slate-400" : "bg-sky-300 text-slate-950"}`}>{item.read ? t(locale, "notify.read") : t(locale, "notify.unreadLabel")}</span>
                      {labelFor(item, locale)}
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
