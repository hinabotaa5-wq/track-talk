"use client";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { t, L } from "@/lib/i18n";
import { useLocale, useLocaleControls } from "@/components/locale-provider";
import { LanguageSelect } from "@/components/language-select";
import { appEvents } from "@/lib/practice-events";
import { profileIncomplete } from "@/lib/profile-gate";
import { trackEvent } from "@/lib/analytics";
import { isReservedNickname, reservedNicknameMessage } from "@/lib/reserved-nickname";
import { nicknameSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import type { z } from "zod";

export default function ProfilePage() {
  const { session, token, profile, loading, refreshProfile } = useAuth();
  // 表示言語（本体 #521・#530）。いつもの変更はマイページ本体の「表示言語」で行う。
  // 登録直後（オンボーディング中）だけここでも選べるようにし、選んだ言語（端末保存）をアカウントの初期値として保存する。
  const locale = useLocale();
  const { chooseDeviceLocale } = useLocaleControls();
  const router = useRouter();
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const form = useForm<z.infer<typeof nicknameSchema>>({
    resolver: zodResolver(nicknameSchema),
    defaultValues: { nickname: "", nickname_public: false, event: "" as never },
  });
  const nickname = useWatch({ control: form.control, name: "nickname" });
  const overLimit = Array.from(nickname).length > 10;

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/profile");
  }, [loading, session, router]);

  useEffect(() => {
    if (!profile) return;
    form.reset({
      nickname: profile.nickname ?? "",
      nickname_public: profile.nickname_public ?? false,
      event: (profile.event ?? "") as never,
    });
  }, [profile, form]);

  async function onSubmit(values: z.infer<typeof nicknameSchema>) {
    if (!token) return;
    setMessage("");
    setError("");
    if (!profile?.is_admin && values.nickname !== profile?.nickname && isReservedNickname(values.nickname)) {
      form.setError("nickname", { type: "reserved", message: reservedNicknameMessage });
      return;
    }
    // ニックネームか種目がまだの人が決め終えたら、ほかの画面へ進めるのでホームへ送る。
    const firstTime = profileIncomplete(profile);
    try {
      await apiFetch("/v1/me", {
        method: "PATCH",
        token,
        body: JSON.stringify(firstTime ? { ...values, locale } : values),
      });
      if (firstTime) trackEvent("onboarding_completed");
      await refreshProfile();
      if (firstTime) {
        router.replace("/");
        return;
      }
      setMessage(t(locale, "profile.saved"));
    } catch (e) {
      if (e instanceof ApiRequestError && e.code === "nickname_reserved") {
        form.setError("nickname", { type: "reserved", message: e.message });
        return;
      }
      setError(e instanceof ApiRequestError ? e.message : L("更新に失敗しました。"));
    }
  }

  const incomplete = profileIncomplete(profile);

  if (loading || !session) {
    return <PageLoading screen label={t(locale, "common.loading")} />;
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{incomplete ? t(locale, "profile.onboardingTitle") : t(locale, "profile.settingsTitle")}</h1>
      {incomplete ? <p className="text-sm text-slate-200">{t(locale, "profile.onboardingHint")}</p> : null}
      {incomplete ? <LanguageSelect value={locale} onChange={chooseDeviceLocale} hint="register.languageHint" /> : null}
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input placeholder={t(locale, "profile.nickname")} aria-label={t(locale, "profile.nickname")} {...form.register("nickname")} />
        {overLimit ? (
          <p className="text-sm text-red-400">{L("10文字以内で入力してください")}</p>
        ) : form.formState.errors.nickname ? (
          <p role="alert" className="text-sm text-red-400">{form.formState.errors.nickname.message}</p>
        ) : null}
        <label className="block text-sm font-medium">
          {t(locale, "profile.event")}
          <select
            className="mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
            {...form.register("event")}
          >
            <option value="" className="text-slate-900">
              {t(locale, "profile.eventPlaceholder")}
            </option>
            {appEvents.map((item) => (
              <option key={item.value} value={item.value} className="text-slate-900">
                {item.label}
              </option>
            ))}
          </select>
        </label>
        {form.formState.errors.event ? <p className="text-sm text-red-400">{form.formState.errors.event.message}</p> : null}
        <label className="flex items-center gap-2 text-sm text-slate-200">
          <input type="checkbox" className="size-4 accent-blue-500" {...form.register("nickname_public")} />
          {t(locale, "profile.nicknamePublic")}
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        {message ? <p className="text-sm text-emerald-400">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
          {t(locale, "common.save")}
        </Button>
      </form>
    </div>
  );
}
