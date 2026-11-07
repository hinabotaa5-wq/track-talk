"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Bell } from "lucide-react";
import { Button } from "@/components/ui/button";
import { afterStartup } from "@/lib/after-startup";
import { trackEvent } from "@/lib/analytics";
import { detectInstallPlatform } from "@/lib/install-guide";
import { pushSupported, readPushSubscription, registerPushSubscription } from "@/lib/push";
import { useActionLock } from "@/lib/use-action-lock";

const snoozeMs = 7 * 24 * 60 * 60 * 1000;
const modalSelector = 'dialog[open], [aria-modal="true"]';
type Status = NotificationPermission | "install" | "unsupported";
type Sheet = "default" | "install" | "denied";

function platform() {
  return detectInstallPlatform(navigator.userAgent, navigator.maxTouchPoints);
}

function permissionStatus(): Status {
  if (window.Notification?.permission === "denied") return "denied";
  if (platform().startsWith("ios") &&
      !(navigator as Navigator & { standalone?: boolean }).standalone &&
      !window.matchMedia("(display-mode: standalone)").matches) return "install";
  return pushSupported() ? Notification.permission : "unsupported";
}

function storageKey(userId: string, kind: "default" | "install") {
  return `tracktalk:push-guide:${kind}:${userId}`;
}

function snoozed(key: string) {
  const until = Number(localStorage.getItem(`${key}:snoozed-until`));
  return Date.now() < until;
}

function PushGuideSheet({
  kind, reviewCompletion, busy, message, onAccept, onClose,
}: {
  kind: Sheet;
  reviewCompletion: boolean;
  busy: boolean;
  message: string;
  onAccept: () => void;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const counted = useRef(false);
  const titleId = useId();
  const bodyId = useId();

  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    element?.querySelector<HTMLElement>("[data-push-guide-close]")?.focus();
    if (!counted.current) {
      counted.current = true;
      trackEvent("push_guide_shown");
    }
    return () => { element?.close(); };
  }, []);

  const denied = kind === "denied";
  const install = kind === "install";
  const device = platform();

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      aria-describedby={bodyId}
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onClose();
      }}
      className="fixed inset-x-0 top-auto bottom-0 m-0 mx-auto max-h-[90dvh] w-full max-w-md overflow-y-auto rounded-t-3xl border border-white/20 bg-slate-900 p-5 pb-[calc(1.25rem+env(safe-area-inset-bottom))] text-slate-100 shadow-2xl backdrop:bg-black/60 sm:inset-y-0 sm:my-auto sm:rounded-3xl sm:pb-5"
    >
      <div className="space-y-3">
        <span className="inline-flex size-10 items-center justify-center rounded-full bg-blue-500/20 text-blue-200">
          <Bell aria-hidden className="size-5" />
        </span>
        <h2 id={titleId} className="text-lg font-bold">
          {denied ? "通知がオフになっています" : install ? "ホーム画面に追加して通知を受け取る" : reviewCompletion ? "返信が届いたら、すぐお知らせします" : "返信やお知らせを、すぐ受け取れます"}
        </h2>
        <div id={bodyId} className="space-y-2 text-sm leading-relaxed text-slate-300">
          {denied ? (
            <ol className="list-decimal space-y-1 pl-5">
              <li>{device === "other" ? "ブラウザで TrackTalk を開く" : "端末の「設定」を開く"}</li>
              <li>{device === "other" ? "アドレスバーのサイト設定を開く" : device === "android" ? "「アプリ」から TrackTalk を選ぶ" : "TrackTalk を選ぶ"}</li>
              <li>「通知」を許可する</li>
            </ol>
          ) : install ? (
            <ol className="list-decimal space-y-1 pl-5">
              <li>{device === "ios-other" ? "Safari で TrackTalk を開き、共有ボタンをタップ" : "Safari の共有ボタンをタップ"}</li>
              <li>「ホーム画面に追加」を選び、「追加」をタップ</li>
              <li>ホーム画面の TrackTalk を開く</li>
            </ol>
          ) : reviewCompletion ? (
            <p>コーチが添削を返したら通知します。あとから設定でいつでも切れます。</p>
          ) : (
            <p>あなたの投稿へのコメントやコメントへの返信、運営からのお知らせがこの端末に届きます。あとからマイページで変更できます。</p>
          )}
        </div>
        {message ? <p role="status" className="text-sm text-slate-200">{message}</p> : null}
        <div className="flex flex-col gap-2 pt-1">
          {kind === "default" ? (
            <Button type="button" className="min-h-11" onClick={onAccept} disabled={busy}>
              通知を受け取る
            </Button>
          ) : null}
          <Button type="button" className="min-h-11" variant="secondary" onClick={onClose} disabled={busy} data-push-guide-close>
            {denied ? "閉じる" : "あとで"}
          </Button>
        </div>
      </div>
    </dialog>
  );
}

export function PushPermissionGuide({
  userId, token, home = false, completedReviewId, onEnabled, renderControl,
}: {
  userId: string;
  token?: string;
  home?: boolean;
  onEnabled?: () => void;
  renderControl?: (status: Status | null, activate: () => void, busy: boolean) => ReactNode;
  completedReviewId?: string;
}) {
  const [status, setStatus] = useState<Status | null>(null);
  const [sheet, setSheet] = useState<Sheet | null>(null);
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    const sync = () => setStatus(permissionStatus());
    sync();
    window.addEventListener("focus", sync);
    window.addEventListener("appinstalled", sync);
    document.addEventListener("visibilitychange", sync);
    return () => {
      window.removeEventListener("focus", sync);
      window.removeEventListener("appinstalled", sync);
      document.removeEventListener("visibilitychange", sync);
    };
  }, []);

  useEffect(() => {
    if ((!home && !completedReviewId) || (status !== "default" && status !== "install")) return;
    const kind = status;
    const key = storageKey(userId, kind);
    let active = true;
    const observer = new MutationObserver(tryOpen);
    function tryOpen() {
      if (!active || document.visibilityState === "hidden" || document.querySelector(modalSelector)) return;
      observer.disconnect();
      document.removeEventListener("visibilitychange", tryOpen);
      try {
        const seenKey = completedReviewId ? `${key}:review:${completedReviewId}` : `${key}:home-seen`;
        if (permissionStatus() !== status || localStorage.getItem(seenKey) || (!completedReviewId && snoozed(key))) return;
        localStorage.setItem(seenKey, "1");
        setSheet(kind);
      } catch {
        // 保存できない環境で毎回自動表示しない。
      }
    }
    const cancel = afterStartup(() => {
      const existing = status === "default" ? readPushSubscription() : Promise.resolve(null);
      void existing.then((subscription) => {
        if (!active || subscription) return;
        observer.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ["open", "aria-modal"] });
        document.addEventListener("visibilitychange", tryOpen);
        tryOpen();
      }).catch(() => {});
    });
    return () => {
      active = false;
      cancel();
      observer.disconnect();
      document.removeEventListener("visibilitychange", tryOpen);
    };
  }, [completedReviewId, home, status, userId]);

  function later() {
    if (sheet === "default" || sheet === "install") {
      try {
        localStorage.setItem(`${storageKey(userId, sheet)}:snoozed-until`, String(Date.now() + snoozeMs));
      } catch {
        // 保存できなくても案内は閉じられる。
      }
      trackEvent("push_guide_later");
    }
    setMessage("");
    setSheet(null);
  }

  async function enable(fromSheet: boolean) {
    const current = permissionStatus();
    if (current !== "granted" && !(fromSheet && current === "default")) {
      setStatus(current);
      setSheet(current === "denied" && !home && !completedReviewId ? "denied" : null);
      return;
    }
    if (!acquire()) return;
    setMessage("");
    try {
      if (fromSheet) trackEvent("push_guide_accept");
      if (current === "default") {
        const result = await Notification.requestPermission();
        setStatus(result);
        if (result === "granted") trackEvent("push_permission_granted");
        if (result === "denied") trackEvent("push_permission_denied");
        if (result !== "granted") {
          setSheet(null);
          return;
        }
      }
      await registerPushSubscription(userId);
      setSheet(null);
      onEnabled?.();
      setMessage("この端末で通知を受け取れるようになりました");
    } catch {
      setMessage("通知を設定できませんでした。もう一度お試しください。");
    } finally {
      release();
    }
  }

  async function activate() {
    const current = permissionStatus();
    setStatus(current);
    setMessage("");
    if (document.querySelector(modalSelector)) return;
    if (current === "denied") {
      setSheet("denied");
      return;
    }
    if (current === "unsupported") return;
    if (current === "granted") {
      await enable(false);
      return;
    }
    try {
      if (snoozed(storageKey(userId, current))) {
        setMessage("「あとで」を選んでから7日経つと、通知を設定できます。");
        return;
      }
    } catch {
      // 保存できなくても手動で設定できる。
    }
    try {
      if (current === "default" && await readPushSubscription()) return;
      if (permissionStatus() === current && !document.querySelector(modalSelector)) setSheet(current);
    } catch {
      setMessage("通知の状態を確認できませんでした。もう一度お試しください。");
    }
  }

  return (
    <>
      {renderControl?.(status, () => void activate(), pending)}
      {sheet ? (
        <PushGuideSheet kind={sheet} reviewCompletion={Boolean(completedReviewId)} busy={pending} message={message} onAccept={() => void enable(true)} onClose={later} />
      ) : message && !home ? (
        <p role="status" className="px-4 py-2 text-sm text-slate-300">{message}</p>
      ) : null}
    </>
  );
}
