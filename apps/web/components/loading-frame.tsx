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

export function FeedLoadingFrame() {
  return (
    <Frame label="投稿を読み込んでいます">
      <div className={feedGridClass}>
        {Array.from({ length: 8 }, (_, index) => (
          <div key={index} className="aspect-[4/3] rounded-2xl bg-zinc-200" />
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
          <div key={index} className="h-28 rounded-xl bg-zinc-200" />
        ))}
      </div>
    </Frame>
  );
}

export function DetailLoadingFrame() {
  return (
    <Frame label="投稿を読み込んでいます">
      <div className="space-y-4">
        <div className="h-8 w-2/3 rounded-md bg-zinc-200" />
        <div className="h-48 rounded-xl bg-zinc-200" />
        <div className="h-4 w-full rounded-md bg-zinc-200" />
        <div className="h-4 w-5/6 rounded-md bg-zinc-200" />
      </div>
    </Frame>
  );
}

export function FormLoadingFrame() {
  return (
    <Frame label="画面を準備しています">
      <div className="space-y-4">
        <div className="h-10 rounded-md bg-zinc-200" />
        <div className="h-32 rounded-md bg-zinc-200" />
        <div className="h-10 rounded-md bg-zinc-200" />
      </div>
    </Frame>
  );
}
