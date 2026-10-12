"use client";

import { useEffect } from "react";
import { useAuth } from "@/components/auth-provider";
import { loadCompetitions } from "@/lib/competition-list";
import { readSavedCompetitionPrefecture } from "@/lib/competition-prefecture";
import { currentMonthInTokyo, INTERNATIONAL } from "@/lib/prefectures";

export function CompetitionPrefetch() {
  const { session, loading } = useAuth();

  useEffect(() => {
    if (loading || !session) return;
    const prefecture = readSavedCompetitionPrefecture() || INTERNATIONAL;
    void loadCompetitions(prefecture, currentMonthInTokyo());
  }, [loading, session]);

  return null;
}
