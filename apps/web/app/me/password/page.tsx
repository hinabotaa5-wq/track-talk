"use client";

import { AccountPageShell, PasswordSection } from "@/components/account-forms";

export default function PasswordPage() {
  return (
    <AccountPageShell title="パスワードの変更" path="/me/password">
      {(email) => <PasswordSection email={email} />}
    </AccountPageShell>
  );
}
