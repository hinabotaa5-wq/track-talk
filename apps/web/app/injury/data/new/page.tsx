import { ComposerPage } from "@/components/composer-page";
import { InjuryCaseForm } from "@/components/injury-case-form";

export const metadata = { title: "ケガの状況を答える" };

export default function InjuryCaseNewPage() {
  return (
    <ComposerPage title="ケガの状況を答える">
      <InjuryCaseForm />
    </ComposerPage>
  );
}
