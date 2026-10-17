import { CoachingGate } from "@/components/coaching-gate";

export default function CoachingSetupLayout({ children }: { children: React.ReactNode }) {
  return <CoachingGate>{children}</CoachingGate>;
}
