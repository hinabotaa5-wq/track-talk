"use client";

import { useEffect, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { FormLoadingFrame } from "@/components/loading-frame";
import { LeaveGuardArea } from "@/components/leave-guard";

export function ComposerPage({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  return (
    <LeaveGuardArea className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">{title}</h1>
      {loading || !session ? <FormLoadingFrame /> : children}
    </LeaveGuardArea>
  );
}
