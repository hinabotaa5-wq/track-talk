"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { cityPhotoSrc, preloadCityPhotos } from "@/lib/city-photo";
import { loadMajorHosts, peekMajorHosts } from "@/lib/major-hosts";
import type { MajorHost } from "@/lib/types";

export function MajorHostsPanel() {
  const { token } = useAuth();
  const [open, setOpen] = useState(false);
  const [hosts, setHosts] = useState<MajorHost[] | null>(peekMajorHosts);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(() => peekMajorHosts() === null);

  useEffect(() => {
    if (!token) return;
    let active = true;
    loadMajorHosts(token)
      .then((next) => {
        if (!active) return;
        preloadCityPhotos(next.map((host) => host.image_url), "high");
        setHosts(next);
        setError("");
      })
      .catch(() => {
        if (active) setError("開催地を取得できませんでした。");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [token]);

  return (
    <div className="space-y-3">
      <Button
        type="button"
        variant="secondary"
        className="h-11 w-full md:w-fit"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        今後の世界陸上・オリンピック
      </Button>
      {open && loading ? <p className="text-sm text-slate-200">読み込み中...</p> : null}
      {open && !loading ? <MajorHostList hosts={hosts ?? []} error={error} /> : null}
    </div>
  );
}

function MajorHostList({ hosts, error }: { hosts: MajorHost[]; error: string }) {
  if (error) return <ErrorState message={error} />;
  if (hosts.length === 0) {
    return <EmptyState title="今後の開催地はまだありません" body="開催地が決まると、ここに表示されます。" />;
  }
  return (
    <div className="space-y-10">
      {hosts.map((host) => {
        const rows = [
          { label: "日程", value: scheduleRange(host.schedule_label) },
          host.venue ? { label: "会場", value: host.venue } : null,
          host.country ? { label: "開催国", value: host.country } : null,
        ].filter((row) => row !== null);
        return (
          <article key={`${host.kind}-${host.year}`} className="space-y-6">
            <HostPhoto url={host.image_url} alt={cityName(host)} />
            <h2 className="text-3xl font-bold text-slate-100">{cityName(host)}</h2>
            <p className="text-sm text-slate-200">
              {host.year}年・{kindLabel(host.kind)}
            </p>
            <div>
              <h3 className="mb-2 text-xl font-semibold text-slate-100">詳細</h3>
              <dl className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
                {rows.map((row) => (
                  <div key={row.label}>
                    <dt className="text-sm text-slate-200">{row.label}</dt>
                    <dd className="mt-1 whitespace-pre-wrap text-slate-100">{row.value}</dd>
                  </div>
                ))}
                {host.page_url ? (
                  <div>
                    <dt className="text-sm text-slate-200">関連ページ</dt>
                    <dd className="mt-1">
                      <a href={host.page_url} target="_blank" rel="noopener noreferrer" className="text-blue-400">
                        Wikipedia
                      </a>
                    </dd>
                  </div>
                ) : null}
              </dl>
            </div>
          </article>
        );
      })}
    </div>
  );
}

function cityName(host: MajorHost) {
  const name = host.city || host.country;
  if (name.endsWith("市") && name.length > 1) return name.slice(0, -1);
  return name;
}

function scheduleRange(label: string) {
  const rest = label.replace(/^\d{4}年/, "");
  if (rest.includes("日")) return rest;
  if (/^\d{1,2}月$/.test(rest)) return `${rest}（日にち未定）`;
  return "日程未定";
}

function HostPhoto({ url, alt }: { url: string; alt: string }) {
  const preferred = cityPhotoSrc(url);
  const [src, setSrc] = useState(preferred);
  const [failed, setFailed] = useState(false);
  const visible = src !== "" && !failed;
  return (
    <div className="relative aspect-[4/3] overflow-hidden rounded-xl bg-black/25">
      {visible ? (
        <>
          <img
            src={src}
            alt={alt}
            decoding="async"
            fetchPriority="high"
            className="h-full w-full object-cover"
            onError={() => {
              if (src !== url && url !== "") {
                setSrc(url);
                return;
              }
              setFailed(true);
            }}
          />
          <p className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/55 to-transparent px-4 pb-2 pt-8 text-xs text-white">
            写真: Wikimedia Commons
          </p>
        </>
      ) : null}
    </div>
  );
}

function kindLabel(kind: MajorHost["kind"]) {
  return kind === "olympics" ? "オリンピック" : "世界陸上";
}
