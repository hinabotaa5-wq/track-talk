import type { Metadata } from "next";
import { PrivacyPolicy } from "@/components/privacy-policy";

export const metadata: Metadata = {
  title: "プライバシーポリシー",
  description: "TrackTalk のプライバシーポリシー",
};

export default function PrivacyPage() {
  return <PrivacyPolicy />;
}
