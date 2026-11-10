"use client";
import { L } from "@/lib/i18n";

import { AccountPageShell, EmailSection } from "@/components/account-forms";

export default function EmailPage() {
  return (
    <AccountPageShell title={L("メールアドレスの変更")} subject={L("メールアドレス")} path="/me/email">
      {(email) => <EmailSection email={email} />}
    </AccountPageShell>
  );
}
