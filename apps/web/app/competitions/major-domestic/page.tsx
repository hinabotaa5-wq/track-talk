import type { Metadata } from "next";
import { MajorDomesticPage } from "./major-domestic-page";

export const metadata: Metadata = {
  title: "国内主要大会",
};

export default function Page() {
  return <MajorDomesticPage />;
}
