"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { preloadCityPhotos } from "@/lib/city-photo";
import { loadCompetitions } from "@/lib/competition-list";
import { readSavedCompetitionPrefecture } from "@/lib/competition-prefecture";
import { loadMajorHosts } from "@/lib/major-hosts";
import { currentMonthInTokyo, INTERNATIONAL } from "@/lib/prefectures";

export function CompetitionPrefetch() {
  const pathname = usePathname();
  const { session, loading } = useAuth();

  useEffect(() => {
    if (loading || !session) return;
    const prefecture = readSavedCompetitionPrefecture() || INTERNATIONAL;
    void loadCompetitions(prefecture, currentMonthInTokyo());
    if (pathname.startsWith("/competitions")) return;
    void loadMajorHosts().then((hosts) => {
      preloadCityPhotos(hosts.map((host) => host.image_url));
    });
  }, [loading, session, pathname]);

  return null;
}
