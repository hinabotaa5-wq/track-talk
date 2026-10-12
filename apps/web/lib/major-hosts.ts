import { apiFetch } from "./api";
import type { MajorHost } from "./types";

let cachedHosts: MajorHost[] | null = null;
let hostsRequest: Promise<MajorHost[]> | null = null;

export function peekMajorHosts() {
  return cachedHosts;
}

export function loadMajorHosts() {
  if (cachedHosts) return Promise.resolve(cachedHosts);
  if (!hostsRequest) {
    hostsRequest = apiFetch<{ hosts: MajorHost[] }>("/v1/major-hosts")
      .then((data) => {
        cachedHosts = data.hosts ?? [];
        return cachedHosts;
      })
      .catch((err) => {
        hostsRequest = null;
        throw err;
      });
  }
  return hostsRequest;
}
