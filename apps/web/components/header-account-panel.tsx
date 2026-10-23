"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { Shield, UserRound } from "lucide-react";
import { useId, useRef, useState } from "react";
import { AnchoredPanel } from "./anchored-panel";

const loading = () => <p className="p-4 text-sm text-slate-200">読み込み中...</p>;
const AccountMenu = dynamic(() => import("@/app/me/page"), { loading });
const AdminMenu = dynamic(() => import("@/app/admin/page"), { loading });

export function HeaderAccountPanel({ admin = false, signedIn }: { admin?: boolean; signedIn: boolean }) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLDivElement>(null);
  const panelID = useId();
  const label = admin ? "管理" : "マイページ";
  const Icon = admin ? Shield : UserRound;

  return (
    <div ref={anchorRef} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-expanded={open}
        aria-controls={panelID}
        onClick={() => setOpen((value) => !value)}
        className={`inline-flex size-10 items-center justify-center rounded-full transition active:scale-[0.98] ${open ? "bg-white/20 text-white" : "text-slate-300 hover:bg-white/16"}`}
      >
        <Icon className="size-5" strokeWidth={2} aria-hidden />
      </button>
      <AnchoredPanel
        open={open}
        anchorRef={anchorRef}
        panelID={panelID}
        onClose={() => setOpen(false)}
        className={`${admin ? "w-[56rem]" : "w-80"} max-w-[calc(100vw-2rem)] max-h-[calc(100dvh-6rem-env(safe-area-inset-top))] overflow-auto rounded-xl border border-white/20 bg-[#16243a] shadow-xl`}
      >
        <div role="dialog" aria-label={label} className="space-y-3 p-3">
          <p className="text-sm font-semibold text-slate-100">{label}</p>
          {signedIn ? (admin ? <AdminMenu /> : <AccountMenu />) : (
            <Link href="/login" className="block rounded-xl bg-white/10 p-3 text-sm text-blue-300">ログインしてマイページを開く</Link>
          )}
        </div>
      </AnchoredPanel>
    </div>
  );
}
