"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { mustCompleteProfile } from "@/lib/profile-gate";
import { useAuth } from "./auth-provider";

// ニックネームと種目を決めるまで、ほかの画面に進ませない。以前から使っている人は、次に開いたときにここで止まる。
export function ProfileGate() {
  const { profile, loading } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const blocked = !loading && mustCompleteProfile(pathname, profile);

  useEffect(() => {
    if (blocked) router.replace("/me/profile?onboarding=1");
  }, [blocked, router]);

  return null;
}
