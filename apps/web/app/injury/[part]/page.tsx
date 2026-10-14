import { notFound } from "next/navigation";
import { isInjuryBodyPart } from "@/lib/injury";
import { GroupView } from "./group-view";

export default async function InjuryGroupPage({ params }: { params: Promise<{ part: string }> }) {
  const { part } = await params;
  if (!isInjuryBodyPart(part)) notFound();
  return <GroupView part={part} />;
}
