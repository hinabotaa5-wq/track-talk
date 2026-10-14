"use client";

import { MajorHostsPanel } from "@/components/major-hosts-panel";
import { RequireSession } from "../require-session";

export function MajorHostsPage() {
  return (
    <RequireSession>
      <MajorHostsPanel />
    </RequireSession>
  );
}
