"use client";

import { useEffect } from "react";

const settleMs = 150;

function typingField(el: Element | null): HTMLInputElement | HTMLTextAreaElement | null {
  if (el instanceof HTMLTextAreaElement) return el;
  if (el instanceof HTMLInputElement && !["checkbox", "radio", "button", "submit", "range", "file", "color"].includes(el.type)) return el;
  return null;
}

// iPhone の Safari では、入力中に画面をスクロールすると、点滅する青い棒(キャレット)が入力欄から離れて画面の上を動いてしまう。
// 特に下に固定したコメント欄で目立つ。スクロールしている間は棒を隠し、止まったら入力欄の中の元の位置に置き直す。
export function CaretGuard() {
  useEffect(() => {
    let field: HTMLInputElement | HTMLTextAreaElement | null = null;
    let timer = 0;

    function settle() {
      timer = 0;
      const el = field;
      field = null;
      if (!el) return;
      el.style.caretColor = "";
      if (document.activeElement !== el) return;
      // 選択の位置を同じ値で置き直すと、Safari が棒を入力欄の中に描き直す。種類によっては選択を持たないので、そのときは何もしない。
      try {
        const { selectionStart, selectionEnd } = el;
        if (selectionStart !== null && selectionEnd !== null) el.setSelectionRange(selectionStart, selectionEnd);
      } catch {
        // email などは選択の位置を持たない
      }
    }

    function onScroll() {
      const el = typingField(document.activeElement);
      if (!el) return;
      if (field !== el) {
        if (field) field.style.caretColor = "";
        field = el;
        el.style.caretColor = "transparent";
      }
      window.clearTimeout(timer);
      timer = window.setTimeout(settle, settleMs);
    }

    window.addEventListener("scroll", onScroll, { capture: true, passive: true });
    return () => {
      window.removeEventListener("scroll", onScroll, { capture: true });
      window.clearTimeout(timer);
      if (field) field.style.caretColor = "";
    };
  }, []);

  return null;
}
