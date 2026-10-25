"use client";

import Link, { useLinkStatus } from "next/link";
import { useEffect, useState, type ComponentProps, type ReactNode } from "react";
import { afterStartup } from "@/lib/after-startup";

function TabContent({ children }: { children: ReactNode }) {
  const { pending } = useLinkStatus();
  return (
    <span aria-busy={pending || undefined} className={`contents ${pending ? "[&>*]:animate-pulse [&>*]:text-blue-300" : ""}`}>
      {children}
    </span>
  );
}

// 起動時の通信を優先し、空き時間か操作の気配があった時点でタブを準備する。
export function MainTabLink({ children, prefetch = null, ...props }: ComponentProps<typeof Link>) {
  const [ready, setReady] = useState(false);
  useEffect(() => afterStartup(() => setReady(true)), []);
  return (
    <Link
      {...props}
      prefetch={ready ? prefetch : false}
      onPointerEnter={(event) => { setReady(true); props.onPointerEnter?.(event); }}
      onPointerDown={(event) => { setReady(true); props.onPointerDown?.(event); }}
      onFocus={(event) => { setReady(true); props.onFocus?.(event); }}
    >
      <TabContent>{children}</TabContent>
    </Link>
  );
}
