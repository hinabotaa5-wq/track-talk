"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { preloadCityPhotos } from "@/lib/city-photo";
import { loadCompetitions } from "@/lib/competition-list";
import { loadDomesticMajors } from "@/lib/domestic-majors";
import { loadMajorHosts } from "@/lib/major-hosts";
import { INTERNATIONAL } from "@/lib/prefectures";

export function CompetitionPrefetch() {
  const pathname = usePathname();
  const { session, loading } = useAuth();

  useEffect(() => {
    // ログインなしで開く人には、先読みの通信をかけない。
    if (loading || !session) return;
    loadDomesticMajors().catch(() => {});
    // 大会タブで「国際大会」に切り替えたとき、読み込みを待たずに出せるよう先に取っておく。
    void loadCompetitions(INTERNATIONAL, 0);
    if (pathname.startsWith("/competitions")) return;
    void loadMajorHosts().then((hosts) => {
      preloadCityPhotos(hosts.map((host) => host.image_url));
    });
  }, [loading, session, pathname]);

  return null;
}
