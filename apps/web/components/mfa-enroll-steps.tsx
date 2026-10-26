"use client";

import { useState, useSyncExternalStore } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const query = "(pointer: coarse)";

function subscribe(onChange: () => void) {
  const media = window.matchMedia(query);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}

// スマホ・タブレット(指で触る端末)か。QR コードは別の端末のカメラで読むものなので、スマホでは出さない。
function useTouchDevice() {
  return useSyncExternalStore(
    subscribe,
    () => window.matchMedia(query).matches,
    () => false,
  );
}

function Step({ n, title, children }: { n: number; title: string; children?: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-blue-600 text-sm font-bold text-white">{n}</span>
      <div className="min-w-0 flex-1 space-y-2">
        <p className="text-sm font-semibold text-slate-100">{title}</p>
        {children}
      </div>
    </div>
  );
}

// 二段階認証の登録を、番号つきの3つの手順で案内する。スマホ1台で設定する人には、QR ではなく「キーをコピー」を出す。
export function MfaEnrollSteps({
  qr,
  secret,
  code,
  onCode,
  pending,
  onSubmit,
}: {
  qr: string;
  secret: string;
  code: string;
  onCode: (value: string) => void;
  pending: boolean;
  onSubmit: (e: React.FormEvent) => void;
}) {
  const touch = useTouchDevice();
  const [showOther, setShowOther] = useState(false);
  const [copied, setCopied] = useState(false);
  const useQr = touch ? showOther : !showOther;

  async function copy() {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-5">
      <Step n={1} title="認証アプリを用意する">
        <p className="text-xs text-slate-300">
          Google Authenticator(無料)を、App Store か Google Play から入れてください。すでに入れている人は次へ進みます。
        </p>
      </Step>

      <Step n={2} title={useQr ? "別の端末のアプリで、この QR コードを読み取る" : "アプリに、下のキーを登録する"}>
        {useQr ? (
          <>
            {/* eslint-disable-next-line @next/next/no-img-element -- Supabase が返す SVG の data URL をそのまま出す */}
            <img src={qr} alt="二段階認証の QR コード" className="mx-auto size-48 rounded-md bg-white p-2" />
            <p className="text-xs text-slate-300">アプリの「＋」→「QR コードをスキャン」でカメラを向けます。</p>
          </>
        ) : (
          <>
            <Button type="button" variant="secondary" className="w-full" onClick={copy}>
              {copied ? "コピーしました" : "キーをコピーする"}
            </Button>
            <p className="break-all rounded-md bg-black/20 p-2 font-mono text-xs text-slate-100">{secret}</p>
            <p className="text-xs text-slate-300">
              アプリを開いて「＋」→「セットアップキーを入力」を選び、アカウント名に「TrackTalk」、キーに貼り付けて追加します。
            </p>
          </>
        )}
        <button type="button" onClick={() => setShowOther((v) => !v)} className="text-xs text-blue-300 underline">
          {useQr ? "QR コードを読み取れない・この端末で設定している" : "別の端末のカメラで QR コードを読み取る"}
        </button>
      </Step>

      <Step n={3} title="アプリに出た6桁の数字を入れる">
        <Input inputMode="numeric" autoComplete="one-time-code" placeholder="123456" maxLength={8} value={code} onChange={(e) => onCode(e.target.value)} />
        <Button type="submit" className="w-full" disabled={pending}>
          設定する
        </Button>
      </Step>
    </form>
  );
}
