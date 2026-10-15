"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { preloadCityPhotos } from "@/lib/city-photo";
import { loadDomesticMajors } from "@/lib/domestic-majors";
import { loadMajorHosts } from "@/lib/major-hosts";

export function CompetitionPrefetch() {
  const pathname = usePathname();
  const { session, token, loading } = useAuth();

  useEffect(() => {
    if (loading || !session || !token) return;
    loadDomesticMajors(token).catch(() => {});
    if (pathname.startsWith("/competitions")) return;
    void loadMajorHosts(token).then((hosts) => {
      preloadCityPhotos(hosts.map((host) => host.image_url));
    });
  }, [loading, session, token, pathname]);

  return null;
}
