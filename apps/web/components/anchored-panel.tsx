"use client";

import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";

const clickShieldMs = 400;

export function AnchoredPanel({
  open,
  anchorRef,
  panelID,
  rightShiftRem = 0,
  className,
  onClose,
  children,
}: {
  open: boolean;
  anchorRef: RefObject<HTMLElement | null>;
  panelID: string;
  rightShiftRem?: number;
  className: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const [pos, setPos] = useState<{ top: number; right: number } | null>(null);
  const blockNextClick = useRef(false);

  useLayoutEffect(() => {
    if (!open) return;
    function place() {
      const anchor = anchorRef.current;
      if (!anchor) return;
      const rect = anchor.getBoundingClientRect();
      const fontSize = Number.parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
      const width = document.documentElement.clientWidth;
      setPos({
        top: rect.bottom + 4,
        right: Math.max(16, width - rect.right - rightShiftRem * fontSize),
      });
    }
    place();
    window.addEventListener("resize", place);
    return () => window.removeEventListener("resize", place);
  }, [open, anchorRef, rightShiftRem]);

  useEffect(() => {
    function onClick(event: MouseEvent) {
      if (!blockNextClick.current) return;
      blockNextClick.current = false;
      event.preventDefault();
      event.stopPropagation();
    }
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, []);

  useEffect(() => {
    if (!open) return;
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  function dismiss(event: ReactMouseEvent<HTMLButtonElement>) {
    event.preventDefault();
    event.stopPropagation();
    // パネルを閉じた直後のクリックが、下の投稿や検索欄へ届かないようにする。
    blockNextClick.current = true;
    window.setTimeout(() => {
      blockNextClick.current = false;
    }, clickShieldMs);
    onClose();
  }

  if (!open || !pos) return null;

  return createPortal(
    <>
      <button
        type="button"
        tabIndex={-1}
        aria-hidden="true"
        className="fixed inset-0 z-[70] cursor-default border-0 bg-transparent p-0"
        onMouseDown={(event) => event.preventDefault()}
        onClick={dismiss}
      />
      <div id={panelID} style={{ top: pos.top, right: pos.right }} className={`fixed z-[80] ${className}`}>
        {children}
      </div>
    </>,
    document.body,
  );
}
