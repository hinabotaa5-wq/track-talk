import { apiFetch } from "./api";
import type { MajorHost } from "./types";

let cachedHosts: MajorHost[] | null = null;
let hostsRequest: Promise<MajorHost[]> | null = null;
// 消したあとに、消す前に始めた読み込みが古い一覧を書き戻さないよう、消すたびに進める。
let generation = 0;

export function peekMajorHosts() {
  return cachedHosts;
}

export function clearMajorHosts() {
  generation++;
  cachedHosts = null;
  hostsRequest = null;
}

export function loadMajorHosts() {
  if (cachedHosts) return Promise.resolve(cachedHosts);
  if (!hostsRequest) {
    const startedAt = generation;
    const request: Promise<MajorHost[]> = apiFetch<{ hosts: MajorHost[] }>("/v1/major-hosts")
      .then((data) => {
        const hosts = data.hosts ?? [];
        if (startedAt === generation) cachedHosts = hosts;
        return hosts;
      })
      .catch((err) => {
        if (hostsRequest === request) hostsRequest = null;
        throw err;
      });
    hostsRequest = request;
  }
  return hostsRequest;
}
