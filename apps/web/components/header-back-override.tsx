"use client";

import { useEffect, useSyncExternalStore } from "react";

type Override = { pathname: string; destination: string };
let current: Override | null = null;
const listeners = new Set<() => void>();
const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
};
const snapshot = () => current;
const serverSnapshot = () => null;
function publish(value: Override | null) {
  current = value;
  listeners.forEach((listener) => listener());
}

/** ページの状態から登録し、条件変更とアンマウントで解除する。 */
export function useHeaderBackOverride(pathname: string, destination: string | null) {
  useEffect(() => {
    if (!destination) return;
    const registration = { pathname, destination };
    publish(registration);
    return () => {
      if (current === registration) publish(null);
    };
  }, [pathname, destination]);
}

/** ヘッダーだけが参照する。ページ切替直後も他画面には適用しない。 */
export function useHeaderBackDestination(pathname: string) {
  const override = useSyncExternalStore(subscribe, snapshot, serverSnapshot);
  return override?.pathname === pathname ? override.destination : null;
}
