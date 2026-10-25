export type VolumePlayer = { setVolume: (volume: number) => void; getVolume: () => number; unMute: () => void; mute: () => void; destroy: () => void };
type YouTubeAPI = { Player: new (frame: HTMLIFrameElement, options: { events: { onReady: (event: { target: VolumePlayer }) => void } }) => VolumePlayer };
declare global { interface Window { YT?: YouTubeAPI; onYouTubeIframeAPIReady?: () => void } }
let loading: Promise<YouTubeAPI> | null = null;
export function loadYouTubeAPI(): Promise<YouTubeAPI> {
  if (window.YT?.Player) return Promise.resolve(window.YT);
  if (loading) return loading;
  loading = new Promise((resolve, reject) => {
    const previous = window.onYouTubeIframeAPIReady;
    const timer = window.setTimeout(() => { loading = null; reject(new Error("YouTube controls unavailable")); }, 10000);
    window.onYouTubeIframeAPIReady = () => { previous?.(); clearTimeout(timer); if (window.YT) resolve(window.YT); };
    const script = document.createElement("script");
    script.src = "https://www.youtube.com/iframe_api";
    script.referrerPolicy = "strict-origin-when-cross-origin";
    script.onerror = () => { clearTimeout(timer); loading = null; reject(new Error("YouTube controls unavailable")); script.remove(); };
    document.head.appendChild(script);
  });
  return loading;
}
