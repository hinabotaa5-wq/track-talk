import { CoachingGate } from "@/components/coaching-gate";

export default function CoachRequestsLayout({ children }: { children: React.ReactNode }) {
  return <CoachingGate>{children}</CoachingGate>;
}
