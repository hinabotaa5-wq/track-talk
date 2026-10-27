"use client";

import Link, { useLinkStatus } from "next/link";
import { useEffect, useId, useState, type ComponentProps, type ReactNode } from "react";
import { afterStartup } from "@/lib/after-startup";
import { markLinkPending, useIsLinkPending } from "@/lib/pending-link";

// 開いている途中のリンクは、指定した待ち時間を超えたら文字とアイコンを薄い空色にする。
// 読み込み中のアニメーションはアイコンではなく画面に出す(ScreenLoader)。
function TabContent({ id, children, delay }: { id: string; children: ReactNode; delay: number }) {
  const { pending: linkPending } = useLinkStatus();
  const markedPending = useIsLinkPending(id);
  const waiting = markedPending || linkPending;
  const [delayed, setDelayed] = useState(false);
  useEffect(() => {
    if (!waiting || delay === 0) return;
    const timer = setTimeout(() => setDelayed(true), delay);
    return () => { clearTimeout(timer); setDelayed(false); };
  }, [waiting, delay]);
  const pending = waiting && (delay === 0 || delayed);
  return (
    <span aria-busy={pending || undefined} className={`contents ${pending ? "[&>*]:text-blue-300" : ""}`}>
      {children}
    </span>
  );
}

// 起動時の通信を優先し、空き時間か操作の気配があった時点でタブを準備する。
export function MainTabLink({ children, prefetch = null, pendingDelay = 0, ...props }: ComponentProps<typeof Link> & { pendingDelay?: number }) {
  const id = useId();
  const [ready, setReady] = useState(false);
  useEffect(() => afterStartup(() => setReady(true)), []);
  return (
    <Link
      {...props}
      data-pending-link=""
      data-screen-loader=""
      data-pending-delay={pendingDelay || undefined}
      prefetch={ready ? prefetch : false}
      onNavigate={(event) => { props.onNavigate?.(event); markLinkPending(id, typeof props.href === "string" ? props.href : props.href.pathname ?? ""); }}
      onPointerEnter={(event) => { setReady(true); props.onPointerEnter?.(event); }}
      onPointerDown={(event) => { setReady(true); props.onPointerDown?.(event); }}
      onFocus={(event) => { setReady(true); props.onFocus?.(event); }}
    >
      <TabContent id={id} delay={pendingDelay}>{children}</TabContent>
    </Link>
  );
}
