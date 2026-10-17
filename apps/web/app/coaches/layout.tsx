import { CoachingGate } from "@/components/coaching-gate";

export default function CoachesLayout({ children }: { children: React.ReactNode }) {
  return <CoachingGate>{children}</CoachingGate>;
}
