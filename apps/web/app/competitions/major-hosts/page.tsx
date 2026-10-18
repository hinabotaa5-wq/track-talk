import type { Metadata } from "next";
import { MajorHostsPanel } from "@/components/major-hosts-panel";
import { apiFetch } from "@/lib/api";
import type { MajorHost } from "@/lib/types";

export const metadata: Metadata = {
  title: "今後の世界陸上・オリンピック",
  description: "これから開かれる世界陸上・オリンピックなどの国際大会の開催地と日程の一覧です。",
  alternates: { canonical: "/competitions/major-hosts" },
};

// 一覧をサーバーで描いておき、検索エンジンが中身を読めるようにする。取れなければ画面側で取り直す。
export default async function Page() {
  const hosts = await apiFetch<{ hosts: MajorHost[] }>("/v1/major-hosts", { signal: AbortSignal.timeout(3000) })
    .then((data) => data.hosts ?? [])
    .catch(() => null);
  return <MajorHostsPanel initialHosts={hosts} />;
}
