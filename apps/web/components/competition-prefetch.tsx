"use client";

import { useEffect } from "react";
import { useAuth } from "@/components/auth-provider";
import { loadCompetitions } from "@/lib/competition-list";
import { currentMonthInTokyo, INTERNATIONAL } from "@/lib/prefectures";

export function CompetitionPrefetch() {
  const { session, loading } = useAuth();

  useEffect(() => {
    if (loading || !session) return;
    void loadCompetitions(INTERNATIONAL, currentMonthInTokyo());
  }, [loading, session]);

  return null;
}
