"use client";

import { useCallback, useRef, useState } from "react";

// state だけだと再描画前の連打をすり抜けるので、ref で同期的にロックする。
export function useActionLock() {
  const locked = useRef(false);
  const [pending, setPending] = useState(false);

  const acquire = useCallback(() => {
    if (locked.current) return false;
    locked.current = true;
    setPending(true);
    return true;
  }, []);

  const release = useCallback(() => {
    locked.current = false;
    setPending(false);
  }, []);

  return { pending, acquire, release };
}
