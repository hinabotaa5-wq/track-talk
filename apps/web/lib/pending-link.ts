import { useSyncExternalStore } from "react";

// タップしたリンクを、次の画面の URL に変わるまで「開いています」の状態にしておく。
// Next.js の useLinkStatus は先読み済みだと出ないため、タップした時点から自分で持つ。
let pendingId: string | null = null;
let timer: ReturnType<typeof setTimeout> | undefined;
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

// いまの画面と同じ URL へのタップは URL が変わらないので、クルクルを出さない。
export function markLinkPending(id: string, href: string) {
  const url = new URL(href, window.location.href);
  if (url.pathname + url.search === window.location.pathname + window.location.search) return;
  pendingId = id;
  clearTimeout(timer);
  // 移動が取り消されたときなどに回り続けないよう、しばらくしたら止める。
  timer = setTimeout(clearLinkPending, 15_000);
  emit();
}

export function clearLinkPending() {
  clearTimeout(timer);
  if (typeof document !== "undefined") {
    document.querySelectorAll("[data-boot-pending]").forEach((link) => link.removeAttribute("data-boot-pending"));
  }
  if (pendingId === null) return;
  pendingId = null;
  emit();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}

export function useIsLinkPending(id: string) {
  return useSyncExternalStore(subscribe, () => pendingId === id, () => false);
}

export function markAppHydrated() {
  (window as Window & { __tracktalkHydrated?: boolean }).__tracktalkHydrated = true;
}
