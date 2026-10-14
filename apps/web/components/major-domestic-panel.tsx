"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { BackButton } from "@/components/major-hosts-panel";
import { ErrorState } from "@/components/empty-state";
import {
  DOMESTIC_CATEGORIES,
  domesticMeetCards,
  domesticMeetsOf,
  isDomesticMajorOver,
  loadDomesticMajors,
  peekDomesticMajors,
  type DomesticMeetCard,
} from "@/lib/domestic-majors";
import type { DomesticMajor } from "@/lib/types";

function todayInTokyo() {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Tokyo" }).format(new Date());
}

export function MajorDomesticPanel() {
  const { token } = useAuth();
  const [majors, setMajors] = useState<DomesticMajor[] | null>(peekDomesticMajors);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(() => peekDomesticMajors() === null);

  useEffect(() => {
    if (!token) return;
    let active = true;
    loadDomesticMajors(token)
      .then((next) => {
        if (!active) return;
        setMajors(next);
        setError("");
      })
      .catch(() => {
        if (active) setError("国内主要大会を取得できませんでした。");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [token]);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">国内主要大会</h1>
      {loading ? <p className="text-sm text-slate-200">読み込み中...</p> : null}
      {!loading && error ? <ErrorState message={error} /> : null}
      {!loading && !error ? <MajorDomesticList majors={majors ?? []} /> : null}
      <BackButton />
    </div>
  );
}

function MajorDomesticList({ majors }: { majors: DomesticMajor[] }) {
  const cards = domesticMeetCards(majors);
  const today = todayInTokyo();
  return (
    <div className="space-y-8">
      {DOMESTIC_CATEGORIES.map((category) => (
        <section key={category.key} className="space-y-3">
          <h2 className="text-xl font-semibold text-slate-100">{category.title}</h2>
          <div className="grid gap-3 md:grid-cols-2">
            {domesticMeetsOf(cards, category.key).map((card) => (
              <MeetCard key={card.slug} card={card} today={today} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function MeetCard({ card, today }: { card: DomesticMeetCard; today: string }) {
  const major = card.major;
  const over = major ? isDomesticMajorOver(major, today) : false;
  const body = (
    <>
      <p className="text-xs text-slate-200">
        {major ? major.schedule_label : "日程未定"}
        {over ? <span className="ml-2 rounded-full bg-white/15 px-2 py-0.5 text-slate-100">終了</span> : null}
      </p>
      <h3 className="text-lg font-semibold text-slate-100">{major?.name ?? card.label}</h3>
      {major?.venue ? <p className="mt-1 text-sm text-slate-200">{major.venue}</p> : null}
      {major?.site_url ? <p className="text-sm text-blue-400">公式サイト</p> : null}
    </>
  );
  const base = "block rounded-xl border border-white/20 glass bg-white/10 p-4";
  if (!major?.site_url) return <div className={`${base} ${major ? "" : "opacity-70"}`}>{body}</div>;
  return (
    <a
      href={major.site_url}
      target="_blank"
      rel="noopener noreferrer"
      className={`${base} transition hover:border-blue-700/70 hover:bg-white/16 active:scale-95 ${over ? "opacity-80" : ""}`}
    >
      {body}
    </a>
  );
}
