"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { Download, X } from "lucide-react";
import {
  detectInstallPlatform,
  installGuideOpenEvent,
  installGuideStorageKey,
  isDismissed,
  type InstallPlatform,
} from "@/lib/install-guide";
import { AppIconMark } from "./app-icon-mark";
import { Button } from "./ui/button";
import { hidesBottomNav } from "./bottom-nav";
import {
  AndroidAddArt,
  AndroidMenuArt,
  IosAddToHomeArt,
  IosConfirmArt,
  IosShareButtonArt,
  OpenInSafariArt,
} from "./install-guide-art";

type InstallPromptEvent = Event & {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
};

type Step = { title: string; body: string; art: ReactNode };

function stepsFor(platform: InstallPlatform): Step[] {
  if (platform === "ios-safari") {
    return [
      { title: "画面下の共有ボタンをタップ", body: "四角から矢印が出ているマークです。", art: <IosShareButtonArt /> },
      { title: "「ホーム画面に追加」を選ぶ", body: "メニューを下にスクロールすると見つかります。", art: <IosAddToHomeArt /> },
      { title: "右上の「追加」をタップ", body: "ホーム画面に TrackTalk のアイコンができます。", art: <IosConfirmArt /> },
    ];
  }
  if (platform === "ios-other") {
    return [
      { title: "まず Safari で開く", body: "いまのブラウザでは追加できません。このページの URL をコピーして、Safari で開き直してください。", art: <OpenInSafariArt /> },
      ...stepsFor("ios-safari"),
    ];
  }
  if (platform === "android") {
    return [
      { title: "右上のメニュー(︙)をタップ", body: "Chrome の画面右上にあります。", art: <AndroidMenuArt /> },
      { title: "「ホーム画面に追加」を選ぶ", body: "「アプリをインストール」と表示される場合もあります。", art: <AndroidAddArt /> },
    ];
  }
  return [];
}

function readDismissedAt() {
  try {
    const raw = localStorage.getItem(installGuideStorageKey);
    return raw === null ? null : Number(raw);
  } catch {
    return null;
  }
}

function saveDismissedAt() {
  try {
    localStorage.setItem(installGuideStorageKey, String(Date.now()));
  } catch {
    // 保存できない環境では、次回も案内を出すだけ。
  }
}

function isStandalone() {
  const nav = navigator as Navigator & { standalone?: boolean };
  return nav.standalone === true || window.matchMedia("(display-mode: standalone)").matches;
}

export function InstallGuide() {
  const pathname = usePathname();
  const [platform, setPlatform] = useState<InstallPlatform | null>(null);
  const [standalone, setStandalone] = useState(false);
  const [dismissed, setDismissed] = useState(true);
  const [open, setOpen] = useState(false);
  const [prompt, setPrompt] = useState<InstallPromptEvent | null>(null);

  useEffect(() => {
    function sync() {
      setPlatform(detectInstallPlatform(navigator.userAgent, navigator.maxTouchPoints));
      setStandalone(isStandalone());
      setDismissed(isDismissed(readDismissedAt(), Date.now()));
    }
    sync();

    function onPrompt(event: Event) {
      event.preventDefault();
      setPrompt(event as InstallPromptEvent);
    }
    function onInstalled() {
      setStandalone(true);
      setOpen(false);
    }
    function onOpen() {
      setOpen(true);
    }
    window.addEventListener("beforeinstallprompt", onPrompt);
    window.addEventListener("appinstalled", onInstalled);
    window.addEventListener(installGuideOpenEvent, onOpen);
    return () => {
      window.removeEventListener("beforeinstallprompt", onPrompt);
      window.removeEventListener("appinstalled", onInstalled);
      window.removeEventListener(installGuideOpenEvent, onOpen);
    };
  }, []);

  const close = useCallback(() => {
    setOpen(false);
    setDismissed(true);
    saveDismissedAt();
  }, []);

  useEffect(() => {
    if (!open) return;
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") close();
    }
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKey);
    };
  }, [open, close]);

  async function install() {
    if (!prompt) return;
    await prompt.prompt();
    const { outcome } = await prompt.userChoice;
    setPrompt(null);
    if (outcome === "accepted") setOpen(false);
  }

  if (platform === null || standalone) return null;

  const steps = stepsFor(platform);
  const showBanner = platform !== "other" && !dismissed && !open;
  const navHidden = hidesBottomNav(pathname);

  return (
    <>
      {showBanner ? (
        <div
          className="fixed inset-x-0 z-40 px-3 md:hidden [body[data-comment-typing]_&]:hidden"
          style={{
            bottom: navHidden
              ? "calc(0.75rem + env(safe-area-inset-bottom))"
              : "calc(3.5rem + 0.5rem + env(safe-area-inset-bottom))",
          }}
        >
          <div className="mx-auto flex max-w-md items-center gap-3 rounded-2xl border border-white/25 glass bg-slate-900/80 p-3 shadow-lg backdrop-blur-xl">
            <AppIconMark className="size-10" />
            <p className="min-w-0 flex-1 text-xs leading-snug text-slate-100">
              ホーム画面に追加すると、アプリのようにすぐ開けます
            </p>
            <Button type="button" onClick={() => setOpen(true)} className="min-h-9 shrink-0 px-3.5 text-xs">
              追加方法
            </Button>
            <button
              type="button"
              onClick={close}
              aria-label="閉じる"
              className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full text-slate-300 hover:bg-white/16 hover:text-white"
            >
              <X className="size-4" aria-hidden />
            </button>
          </div>
        </div>
      ) : null}

      {open ? (
        <div className="fixed inset-0 z-50 flex items-end justify-center sm:items-center" role="dialog" aria-modal="true" aria-labelledby="install-guide-title">
          <button type="button" aria-label="閉じる" onClick={close} className="absolute inset-0 cursor-default bg-black/60" />
          <div
            className="relative max-h-[90dvh] w-full max-w-md overflow-y-auto rounded-t-3xl border border-white/20 bg-slate-900 p-5 shadow-2xl sm:rounded-3xl"
            style={{ paddingBottom: "calc(1.25rem + env(safe-area-inset-bottom))" }}
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 id="install-guide-title" className="text-lg font-semibold text-white">
                  ホーム画面に追加して使う
                </h2>
                <p className="mt-1 text-xs text-slate-300">アプリのようにワンタップで開けます。無料で、数十秒で終わります。</p>
              </div>
              <button
                type="button"
                onClick={close}
                aria-label="閉じる"
                className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full text-slate-300 hover:bg-white/16 hover:text-white"
              >
                <X className="size-5" aria-hidden />
              </button>
            </div>

            {platform === "other" ? (
              <p className="mt-5 rounded-xl border border-white/15 bg-white/8 p-4 text-sm text-slate-200">
                スマホのブラウザ(iPhone は Safari、Android は Chrome)でこのページを開くと、ホーム画面に追加できます。
              </p>
            ) : (
              <>
                {platform === "android" && prompt ? (
                  <Button type="button" onClick={install} className="mt-4 w-full gap-2 py-3">
                    <Download className="size-4" aria-hidden />
                    このままアプリとして追加する
                  </Button>
                ) : null}
                <ol className="mt-5 space-y-5">
                  {steps.map((step, index) => (
                    <li key={step.title}>
                      <div className="mb-2 flex items-start gap-2.5">
                        <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-blue-600 text-xs font-bold text-white">
                          {index + 1}
                        </span>
                        <div>
                          <p className="text-sm font-semibold text-white">{step.title}</p>
                          <p className="mt-0.5 text-xs text-slate-300">{step.body}</p>
                        </div>
                      </div>
                      {step.art}
                    </li>
                  ))}
                </ol>
              </>
            )}

            <Button type="button" variant="secondary" onClick={close} className="mt-6 w-full py-3">
              あとで
            </Button>
          </div>
        </div>
      ) : null}
    </>
  );
}
