"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef } from "react";
import { apiFetch } from "@/lib/api";
import { overseasConsentVersion, signupConsentVersion, takeOverseasConsent } from "@/lib/overseas-consent";
import { mustCompleteProfile, mustConsentOverseas } from "@/lib/profile-gate";
import { useAuth } from "./auth-provider";

// 海外の事業者に預けることへの同意と、ニックネームと種目を決めるまで、ほかの画面に進ませない。
// 以前から使っている人は、次に開いたときにここで止まる。
export function ProfileGate() {
  const { session, profile, loading, token, refreshProfile } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const tried = useRef(false);
  const recording = useRef(false);
  const consentMissing = !loading && profile?.overseas_consent === false;
  const needsConsent = mustConsentOverseas(pathname, profile);
  const blocked = !loading && !consentMissing && mustCompleteProfile(pathname, profile);

  useEffect(() => {
    if (consentMissing && !tried.current) {
      tried.current = true;
      // 登録画面で同意して登録した人は、もう一度聞かずに記録する。
      // 確認メールを別のタブで開いた人は、アカウントに残した版で記録する(いまの版と違えば API が断り、同意の画面へ)。
      const fromTab = takeOverseasConsent();
      const version = signupConsentVersion(session?.user?.user_metadata) ?? (fromTab ? overseasConsentVersion : null);
      if (version) {
        recording.current = true;
        void apiFetch<void>("/v1/me/consents/overseas", { method: "POST", token, body: JSON.stringify({ version }) })
          .catch(() => {})
          .finally(() => {
            // 読み直した結果で、もう一度ここを通る(記録できなかった人は同意の画面へ)。
            recording.current = false;
            void refreshProfile();
          });
      }
    }
    if (recording.current) return;
    if (needsConsent) router.replace(`/consent?next=${encodeURIComponent(pathname)}`);
    else if (blocked) router.replace("/me/profile?onboarding=1");
  }, [consentMissing, needsConsent, blocked, pathname, router, session, token, refreshProfile]);

  return null;
}
