"use client";

import { AccountPageShell, EmailSection } from "@/components/account-forms";

export default function EmailPage() {
  return (
    <AccountPageShell title="メールアドレスの変更" path="/me/email">
      {(email) => <EmailSection email={email} />}
    </AccountPageShell>
  );
}
