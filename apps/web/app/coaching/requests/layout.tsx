import { CoachingGate } from "@/components/coaching-gate";

export default function ReviewRequestsLayout({ children }: { children: React.ReactNode }) {
  return <CoachingGate>{children}</CoachingGate>;
}
