"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { preloadCityPhotos } from "@/lib/city-photo";
import { loadCompetitions } from "@/lib/competition-list";
import { loadDomesticMajors } from "@/lib/domestic-majors";
import { loadMajorHosts } from "@/lib/major-hosts";
import { INTERNATIONAL } from "@/lib/prefectures";
import { afterStartup } from "@/lib/after-startup";

export function CompetitionPrefetch() {
  const pathname = usePathname();
  const { session, loading } = useAuth();

  useEffect(() => {
    // ログインなしで開く人には、先読みの通信をかけない。
    if (loading || !session) return;
    return afterStartup(() => {
      loadDomesticMajors().catch(() => {});
      // 大会タブで「国際大会」に切り替えたとき、読み込みを待たずに出せるよう先に取っておく。
      void loadCompetitions(INTERNATIONAL, 0);
      if (pathname.startsWith("/competitions")) return;
      // 先読みの失敗は、使うときに読み直すので捨てる（未処理の拒否にしない）。
      loadMajorHosts()
        .then((hosts) => {
          preloadCityPhotos(hosts.map((host) => host.image_url));
        })
        .catch(() => {});
    });
  }, [loading, session, pathname]);

  return null;
}
