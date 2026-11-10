"use client";
import { L } from "@/lib/i18n";

import { AccountPageShell, PasswordSection } from "@/components/account-forms";

export default function PasswordPage() {
  return (
    <AccountPageShell title={L("パスワードの変更")} subject={L("パスワード")} path="/me/password">
      {(email) => <PasswordSection email={email} />}
    </AccountPageShell>
  );
}
