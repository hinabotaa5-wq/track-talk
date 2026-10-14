"use client";

import { MajorDomesticPanel } from "@/components/major-domestic-panel";
import { RequireSession } from "../require-session";

export function MajorDomesticPage() {
  return (
    <RequireSession>
      <MajorDomesticPanel />
    </RequireSession>
  );
}
