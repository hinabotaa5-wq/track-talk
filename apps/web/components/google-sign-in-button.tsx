"use client";

import { authErrorMessage } from "@/lib/auth-errors";
import { createClient } from "@/lib/supabase";
import { Button } from "@/components/ui/button";
import { useState } from "react";

export function GoogleSignInButton() {
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function signIn() {
    const supabase = createClient();
    if (!supabase) {
      setError("Supabase の公開設定がありません。");
      return;
    }
    setError("");
    setPending(true);
    const { error: err } = await supabase.auth.signInWithOAuth({
      provider: "google",
      options: { redirectTo: `${window.location.origin}/auth/callback` },
    });
    if (err) {
      setPending(false);
      setError(authErrorMessage(err));
    }
  }

  return (
    <div className="space-y-2">
      {error ? <p className="text-sm text-red-600">{error}</p> : null}
      <Button
        type="button"
        variant="secondary"
        className="w-full md:w-auto"
        disabled={pending}
        onClick={signIn}
      >
        Googleでログイン
      </Button>
    </div>
  );
}
