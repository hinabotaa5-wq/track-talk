import type { Metadata } from "next";
import { MajorHostsPage } from "./major-hosts-page";

export const metadata: Metadata = {
  title: "今後の世界陸上・オリンピック",
};

export default function Page() {
  return <MajorHostsPage />;
}
