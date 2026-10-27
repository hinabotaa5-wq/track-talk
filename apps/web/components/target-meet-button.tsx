"use client";

import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { Check, Plus } from "lucide-react";
import { useAuth } from "@/components/auth-provider";
import { apiErrorText } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { clearTargetMeet, setTargetMeet, useTargetMeet, type TargetMeet } from "@/lib/target-meet";

// 大会のカードの右上に付ける「+」。セットした大会はメモタブの瓶の横に「あと何日」で出る。
// 1人1大会なので、別の大会をセットすると置き換わる。セット中のものをもう一度押すと解除する。
export function TargetMeetButton({ source, meetRef, name, className = "" }: { source: TargetMeet["source"]; meetRef: string; name: string; className?: string }) {
  const { session, token } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const { target } = useTargetMeet(session ? token : null);
  const { pending, acquire, release } = useActionLock();
  const [error, setError] = useState("");
  const selected = target?.source === source && target.ref === meetRef;

  async function toggle() {
    if (!session || !token) {
      router.push(`/login?next=${encodeURIComponent(pathname + window.location.search)}`);
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      if (selected) await clearTargetMeet(token);
      else await setTargetMeet(token, source, meetRef);
    } catch (e) {
      setError(apiErrorText(e, "セットできませんでした"));
    } finally {
      release();
    }
  }

  // 見た目はメモタブの瓶の横の「+」と同じ丸。セット中はチェックにする。
  return (
    <div className={`flex flex-col items-end ${className}`}>
      <button
        type="button"
        onClick={toggle}
        disabled={pending}
        aria-pressed={selected}
        aria-label={selected ? `${name}のセットを解除する` : `${name}を目標の大会にセットする`}
        className="-m-1 rounded-full p-1 transition active:scale-[0.94] disabled:opacity-60"
      >
        <span
          className={`flex size-7 items-center justify-center rounded-full ${
            selected ? "bg-sky-400 text-slate-950" : "bg-sky-400/25 text-sky-100 hover:bg-sky-400/40"
          }`}
        >
          {selected ? <Check className="size-4" aria-hidden /> : <Plus className="size-4" aria-hidden />}
        </span>
      </button>
      {error ? <p role="alert" className="mt-1 max-w-32 text-right text-xs text-red-300">{error}</p> : null}
    </div>
  );
}
