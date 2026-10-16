import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { apiFetch } from "@/lib/api";
import { InjuryPublicSummary } from "@/components/injury-public-summary";
import { injuryBodyPartLabel, injuryGroupTitle, isInjuryBodyPart, type InjuryPublicGroup } from "@/lib/injury";
import { GroupView } from "./group-view";

type Props = { params: Promise<{ part: string }> };

async function loadPublicGroup(part: string) {
  try {
    return await apiFetch<InjuryPublicGroup>(`/v1/injury/groups/${part}/public`);
  } catch {
    // 集計が読めなくても、グループに入る・中を読むことはできる。
    return null;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { part } = await params;
  if (!isInjuryBodyPart(part)) return {};
  const title = `${injuryGroupTitle(part)}｜ケガ部`;
  const subject = part === "other" ? "ケガ" : injuryBodyPartLabel(part);
  const description = `${subject}をした陸上選手が集まるグループ。復帰までの目安、ケガをしたときの状況、みんなが意識的にやったこと、次の人への一言を見られます。`;
  return { title, description, alternates: { canonical: `/injury/${part}` }, openGraph: { title, description } };
}

export default async function InjuryGroupPage({ params }: Props) {
  const { part } = await params;
  if (!isInjuryBodyPart(part)) notFound();
  const publicGroup = await loadPublicGroup(part);
  return (
    <GroupView
      part={part}
      members={publicGroup?.members ?? null}
      summary={publicGroup ? <InjuryPublicSummary group={publicGroup} /> : null}
    />
  );
}
