"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { hidesTabBar } from "./bottom-nav";

export function LegalFooter() {
  const pathname = usePathname();
  const aboveTabBar = !hidesTabBar(pathname);

  return (
    <footer
      className={
        aboveTabBar
          ? "sticky bottom-[calc(3.5rem+8px+env(safe-area-inset-bottom))] z-20 bg-background pt-6 md:static md:bg-transparent md:pt-10"
          : "pt-10"
      }
    >
      <nav aria-label="規約" className="flex flex-wrap gap-x-4 gap-y-2 text-sm text-zinc-500">
        <Link href="/about#guidelines">ガイドライン</Link>
        <Link href="/about#terms">利用規約</Link>
        <Link href="/about#privacy">プライバシーポリシー</Link>
      </nav>
    </footer>
  );
}
