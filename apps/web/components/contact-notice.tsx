"use client";

import { contactNoticeText, looksLikeSolicitation, solicitationWarningText } from "@/lib/solicitation";
import { useRef, useState } from "react";

// ContactNotice は、コーチングのテキストの接点に常に出す注意文。
export function ContactNotice({ className = "" }: { className?: string }) {
  return <span className={`block text-xs text-slate-300 ${className}`}>{contactNoticeText}</span>;
}

// useSolicitationGuard は、勧誘に使われやすい言葉があるとき、最初の送信だけ止めて警告する。
// 同じ文のまま、もう一度押すと送る（警告だけで、送信は止めない）。
export function useSolicitationGuard() {
  const confirmed = useRef<string | null>(null);
  const [warning, setWarning] = useState(false);
  function allow(text: string): boolean {
    if (!looksLikeSolicitation(text)) {
      setWarning(false);
      return true;
    }
    if (confirmed.current === text) {
      setWarning(false);
      return true;
    }
    confirmed.current = text;
    setWarning(true);
    return false;
  }
  const node = warning ? <p role="alert" className="rounded-lg border border-amber-400/40 bg-amber-500/15 p-2 text-sm text-amber-100">{solicitationWarningText}</p> : null;
  return { allow, warning: node };
}
