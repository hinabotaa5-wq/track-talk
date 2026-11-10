"use client";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { parseLocale, readStoredLocale, storeLocale, t, type Locale } from "@/lib/i18n";
import { appEvents } from "@/lib/practice-events";
import { profileIncomplete } from "@/lib/profile-gate";
import { trackEvent } from "@/lib/analytics";
import { isReservedNickname, reservedNicknameMessage } from "@/lib/reserved-nickname";
import { nicknameSchema } from "@/lib/validators";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import type { z } from "zod";

export default function ProfilePage() {
  const { session, token, profile, loading, refreshProfile } = useAuth();
  // 言語の選択は選んだそばから画面に反映し、保存で確定する（本体 #521）。
  // 初期値は登録時の選択（端末保存）。本人の保存値が読めたらそちらに寄せるが、選び直しは上書きしない。
  const [locale, setLocale] = useState<Locale>(() => readStoredLocale());
  const touched = useRef(false);
  useEffect(() => {
    if (!profile || touched.current) return;
    const saved = parseLocale(profile.locale);
    if (saved === locale) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLocale(saved);
  }, [profile, locale]);
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
        body: JSON.stringify({ ...values, locale }),
      });
      if (firstTime) trackEvent("onboarding_completed");
      storeLocale(locale);
      await refreshProfile();
      if (firstTime) {
        router.replace("/");
        return;
      }
      setMessage(t(parseLocale(locale), "profile.saved"));
    } catch (e) {
      if (e instanceof ApiRequestError && e.code === "nickname_reserved") {
        form.setError("nickname", { type: "reserved", message: e.message });
        return;
      }
      setError(e instanceof ApiRequestError ? e.message : "更新に失敗しました。");
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
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
        <Input placeholder={t(locale, "profile.nickname")} aria-label={t(locale, "profile.nickname")} {...form.register("nickname")} />
        {overLimit ? (
          <p className="text-sm text-red-400">10文字以内で入力してください</p>
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
        <label className="block text-sm font-medium">
          {t(locale, "profile.language")}
          <select
            className="mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
            value={locale}
            onChange={(e) => {
              touched.current = true;
              setLocale(parseLocale(e.target.value));
            }}
          >
            <option value="ja" className="text-slate-900">{t(locale, "register.language.ja")}</option>
            <option value="en" className="text-slate-900">{t(locale, "register.language.en")}</option>
          </select>
          <span className="mt-1 block text-xs font-normal text-slate-300">{t(locale, "profile.languageHint")}</span>
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
