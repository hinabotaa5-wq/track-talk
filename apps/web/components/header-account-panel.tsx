"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { rememberAccountOrigin, readAccountOrigin } from "@/lib/account-origin";
import { UserRound } from "lucide-react";

export function HeaderAccountPanel({ signedIn }: { signedIn: boolean }) {
  const pathname = usePathname();
  const router = useRouter();
  const active = pathname === "/me" || pathname.startsWith("/me/") || pathname === "/admin" || pathname.startsWith("/admin/");
  return (
    <Link
      href={signedIn ? "/me" : "/login"}
      aria-label="マイページ"
      aria-expanded={pathname === "/me"}
      onClick={(event) => {
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return;
        if (pathname === "/me") {
          event.preventDefault();
          router.replace(readAccountOrigin());
        } else {
          rememberAccountOrigin(window.location.pathname + window.location.search);
        }
      }}
      className={`inline-flex size-10 items-center justify-center rounded-full transition active:scale-[0.98] ${active ? "bg-white/20 text-white" : "text-slate-300 hover:bg-white/16"}`}
    >
      <UserRound className="size-5" strokeWidth={2} aria-hidden />
    </Link>
  );
}
