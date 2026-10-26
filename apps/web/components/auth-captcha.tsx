"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// 登録・ログイン・パスワード再設定の自動操作対策(Cloudflare Turnstile)。
// サイトキーが無い間は何も出さず、トークンも送らない。Supabase 側で CAPTCHA を有効にするときに、
// NEXT_PUBLIC_TURNSTILE_SITE_KEY を入れて画面を出してから、Supabase の設定を切り替える。
export const captchaSiteKey = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";

type Turnstile = {
  render: (el: HTMLElement, options: Record<string, unknown>) => string;
  reset: (id: string) => void;
  remove: (id: string) => void;
};

declare global {
  interface Window {
    turnstile?: Turnstile;
  }
}

const scriptURL = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
let scriptPromise: Promise<Turnstile> | null = null;

function loadTurnstile(): Promise<Turnstile> {
  if (window.turnstile) return Promise.resolve(window.turnstile);
  scriptPromise ??= new Promise<Turnstile>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = scriptURL;
    script.async = true;
    script.onload = () => (window.turnstile ? resolve(window.turnstile) : reject(new Error("turnstile")));
    script.onerror = () => {
      scriptPromise = null;
      reject(new Error("turnstile"));
    };
    document.head.appendChild(script);
  });
  return scriptPromise;
}

// フォームで使う。required が false のとき(サイトキー無し)は token を待たない。
// トークンは1回しか使えないので、送ったあとは成否にかかわらず reset() して取り直す。
export function useCaptcha() {
  const [token, setToken] = useState("");
  const [resetKey, setResetKey] = useState(0);
  const reset = useCallback(() => {
    setToken("");
    setResetKey((n) => n + 1);
  }, []);
  const required = captchaSiteKey !== "";
  return {
    required,
    token,
    ready: !required || token !== "",
    // Supabase に渡す値。サイトキーが無い間は送らない。
    captchaToken: required && token ? token : undefined,
    reset,
    widget: required ? <AuthCaptcha onToken={setToken} resetKey={resetKey} /> : null,
  };
}

export function AuthCaptcha({ onToken, resetKey }: { onToken: (token: string) => void; resetKey: number }) {
  const ref = useRef<HTMLDivElement>(null);
  const widgetId = useRef<string | null>(null);
  const tokenRef = useRef(onToken);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    tokenRef.current = onToken;
  }, [onToken]);

  useEffect(() => {
    let active = true;
    loadTurnstile()
      .then((turnstile) => {
        if (!active || !ref.current || widgetId.current) return;
        widgetId.current = turnstile.render(ref.current, {
          sitekey: captchaSiteKey,
          language: "ja",
          theme: "dark",
          callback: (token: string) => tokenRef.current(token),
          "expired-callback": () => tokenRef.current(""),
          "error-callback": () => tokenRef.current(""),
        });
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
      if (widgetId.current) window.turnstile?.remove(widgetId.current);
      widgetId.current = null;
    };
  }, []);

  useEffect(() => {
    if (resetKey > 0 && widgetId.current) window.turnstile?.reset(widgetId.current);
  }, [resetKey]);

  return (
    <div className="space-y-1">
      <div ref={ref} className="min-h-[65px]" />
      {failed ? <p className="text-sm text-red-400">確認の読み込みに失敗しました。ページを読み直してください。</p> : null}
    </div>
  );
}
