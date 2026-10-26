"use client";

import Link, { useLinkStatus } from "next/link";
import { LoaderCircle } from "lucide-react";
import { createContext, useContext, useEffect, useId, useState, type ComponentProps, type ReactNode } from "react";
import { afterStartup } from "@/lib/after-startup";
import { markLinkPending, useIsLinkPending } from "@/lib/pending-link";

const PendingContext = createContext(false);

function TabContent({ id, children, startup }: { id: string; children: ReactNode; startup: boolean }) {
  const { pending: linkPending } = useLinkStatus();
  const markedPending = useIsLinkPending(id);
  const pending = startup && (markedPending || linkPending);
  return (
    <PendingContext.Provider value={pending}>
      {/* クルクルに替わるアイコン(svg)は点滅させない。animation が重なると回らなくなる。 */}
      <span aria-busy={pending || undefined} className={`contents ${pending ? "[&>*:not(svg)]:animate-pulse [&>*]:text-blue-300" : ""}`}>
        {children}
      </span>
    </PendingContext.Provider>
  );
}

// リンクのアイコン。次の画面が開くまではクルクルに替える。
// React が動く前のタップは、起動スクリプトが付ける data-boot-pending を CSS で見て替える。
export function PendingIcon({ children, className, label }: { children: ReactNode; className: string; label: string }) {
  const pending = useContext(PendingContext);
  if (pending) {
    return (
      <>
        <LoaderCircle className={`${className} animate-spin`} aria-hidden />
        <span role="status" className="sr-only">{label}</span>
      </>
    );
  }
  return (
    <>
      <span className="pending-link-idle contents">{children}</span>
      <LoaderCircle className={`pending-link-boot hidden ${className} animate-spin`} aria-hidden />
    </>
  );
}

// 起動時の通信を優先し、空き時間か操作の気配があった時点でタブを準備する。
export function MainTabLink({ children, prefetch = null, startupOnly = false, ...props }: ComponentProps<typeof Link> & { startupOnly?: boolean }) {
  const id = useId();
  const [ready, setReady] = useState(false);
  const [startup, setStartup] = useState(true);
  useEffect(() => afterStartup(() => { setReady(true); setStartup(false); }), []);
  return (
    <Link
      {...props}
      data-pending-link=""
      prefetch={ready ? prefetch : false}
      onNavigate={(event) => { props.onNavigate?.(event); markLinkPending(id, typeof props.href === "string" ? props.href : props.href.pathname ?? ""); }}
      onPointerEnter={(event) => { setReady(true); props.onPointerEnter?.(event); }}
      onPointerDown={(event) => { setReady(true); props.onPointerDown?.(event); }}
      onFocus={(event) => { setReady(true); props.onFocus?.(event); }}
    >
      <TabContent id={id} startup={!startupOnly || startup}>{children}</TabContent>
    </Link>
  );
}
