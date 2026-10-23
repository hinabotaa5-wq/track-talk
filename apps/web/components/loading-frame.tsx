import type { ReactNode } from "react";
import { feedGridClass } from "@/components/feed-card";

function Frame({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div role="status" aria-busy="true">
      <p className="sr-only">{label}</p>
      {children}
    </div>
  );
}

export function FeedLoadingFrame({ horizontal = false }: { horizontal?: boolean }) {
  if (horizontal) return (
    <Frame label="投稿を読み込んでいます">
      <div className="horizontal-feed-track" aria-hidden>
        {Array.from({ length: 4 }, (_, index) => <div key={index} className="horizontal-feed-slide rounded-2xl glass bg-white/12 animate-pulse" />)}
      </div>
    </Frame>
  );
  return (
    <Frame label="投稿を読み込んでいます">
      <div className={feedGridClass}>
        {["h-40", "h-64", "h-48", "h-72", "h-36", "h-56", "h-44", "h-60"].map((height, index) => (
          <div key={index} className={`mb-3 break-inside-avoid rounded-2xl glass bg-white/12 animate-pulse ${height}`} />
        ))}
      </div>
    </Frame>
  );
}

export function CompetitionLoadingFrame() {
  return (
    <Frame label="大会を読み込んでいます">
      <div className="grid gap-3 md:grid-cols-2">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="h-28 rounded-xl glass bg-white/12 animate-pulse" />
        ))}
      </div>
    </Frame>
  );
}

export function DetailLoadingFrame() {
  return (
    <Frame label="投稿を読み込んでいます">
      <div className="space-y-4">
        <div className="h-8 w-2/3 rounded-md glass bg-white/12 animate-pulse" />
        <div className="h-48 rounded-xl glass bg-white/12 animate-pulse" />
        <div className="h-4 w-full rounded-md glass bg-white/12 animate-pulse" />
        <div className="h-4 w-5/6 rounded-md glass bg-white/12 animate-pulse" />
      </div>
    </Frame>
  );
}

export function FormLoadingFrame() {
  return (
    <Frame label="画面を準備しています">
      <div className="space-y-4">
        <div className="h-10 rounded-md glass bg-white/12 animate-pulse" />
        <div className="h-32 rounded-md glass bg-white/12 animate-pulse" />
        <div className="h-10 rounded-md glass bg-white/12 animate-pulse" />
      </div>
    </Frame>
  );
}
