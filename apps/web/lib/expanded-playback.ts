type FullscreenVideo = HTMLVideoElement & {
  webkitEnterFullscreen?: () => void;
};

type Session = {
  postId: string;
  root: HTMLDivElement;
  media: HTMLVideoElement | HTMLIFrameElement;
  holding: boolean;
  docked: boolean;
  wantsPlay: boolean;
  standardFullscreen: boolean;
  subscribers: Set<() => void>;
};

let session: Session | null = null;
let previousOverflow = "";
let listening = false;

function styleCover(root: HTMLElement) {
  root.style.cssText = "position:fixed;inset:0;z-index:80;display:flex;background:#000;";
}

function styleMediaCover(media: HTMLElement) {
  media.style.cssText = "width:100%;height:100%;border:0;background:#000;object-fit:contain;";
}

function styleMediaSlot(media: HTMLElement) {
  media.style.cssText = "display:block;width:100%;height:100%;border:0;background:#000;object-fit:contain;";
}

function closeButton() {
  const button = document.createElement("button");
  button.type = "button";
  button.setAttribute("aria-label", "閉じる");
  button.textContent = "閉じる";
  button.style.cssText =
    "position:absolute;z-index:2;top:max(0.75rem,env(safe-area-inset-top));left:0.75rem;border:0;border-radius:999px;background:rgba(15,23,42,0.82);color:#f8fafc;padding:0.45rem 0.8rem;font:600 14px/1.2 sans-serif;";
  button.addEventListener("click", () => releaseHold());
  return button;
}

function lockScroll() {
  previousOverflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
}

function unlockScroll() {
  document.body.style.overflow = previousOverflow;
}

function ensureListener() {
  if (listening) return;
  listening = true;
  document.addEventListener("fullscreenchange", () => {
    if (!session?.holding || !session.standardFullscreen || document.fullscreenElement) return;
    releaseHold();
  });
}

function notify() {
  if (!session || session.holding) return;
  for (const subscriber of [...session.subscribers]) subscriber();
}

function resumePlayback(media: HTMLVideoElement | HTMLIFrameElement) {
  if (!(media instanceof HTMLVideoElement) || media.ended) return;
  void media.play().catch(() => {});
}

function releaseHold() {
  if (!session?.holding) return;
  const media = session.media;
  const wantsPlay = media instanceof HTMLVideoElement && !media.paused && !media.ended;
  session.wantsPlay = wantsPlay;
  if (wantsPlay && media instanceof HTMLVideoElement) {
    const until = performance.now() + 1500;
    const onPause = () => {
      if (!session?.wantsPlay || performance.now() > until) {
        media.removeEventListener("pause", onPause);
        return;
      }
      resumePlayback(media);
    };
    media.addEventListener("pause", onPause);
  }
  session.holding = false;
  unlockScroll();
  const root = session.root;
  if (document.fullscreenElement === root) void document.exitFullscreen();
  if (root.isConnected) root.remove();
  notify();
  if (wantsPlay) resumePlayback(media);
}

function clearSession() {
  if (!session) return;
  if (session.media instanceof HTMLVideoElement) session.media.pause();
  session.root.remove();
  if (session.media.isConnected) session.media.remove();
  session.subscribers.clear();
  session = null;
  unlockScroll();
}

function begin(postId: string, label: string, media: HTMLVideoElement | HTMLIFrameElement) {
  clearSession();
  ensureListener();
  const root = document.createElement("div");
  root.setAttribute("role", "dialog");
  root.setAttribute("aria-modal", "true");
  root.setAttribute("aria-label", label);
  styleCover(root);
  styleMediaCover(media);
  root.append(closeButton(), media);
  document.body.appendChild(root);
  session = {
    postId,
    root,
    media,
    holding: true,
    docked: false,
    wantsPlay: false,
    standardFullscreen: false,
    subscribers: new Set(),
  };
  lockScroll();
  return root;
}

export function hasExpandedPlayback(postId: string) {
  return session?.postId === postId;
}

export function openExpandedVideo(postId: string, src: string, label: string, poster?: string) {
  const video = document.createElement("video");
  video.src = src;
  video.controls = true;
  video.playsInline = true;
  video.preload = "auto";
  video.setAttribute("playsinline", "");
  if (poster) video.poster = poster;
  const root = begin(postId, label, video);
  video.addEventListener("webkitendfullscreen", () => releaseHold());
  const fullscreenVideo = video as FullscreenVideo;
  if (typeof fullscreenVideo.webkitEnterFullscreen === "function") {
    try {
      fullscreenVideo.webkitEnterFullscreen();
    } catch {
      // 端末の全画面に入れないときは、画面を覆う再生を閉じるまで続ける。
    }
  } else if (root.requestFullscreen && session) {
    session.standardFullscreen = true;
    void root.requestFullscreen().catch(() => {});
  }
  void video.play().catch(() => {});
}

export function openExpandedYouTube(postId: string, videoId: string, label: string) {
  const iframe = document.createElement("iframe");
  iframe.src = `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoId)}?autoplay=1`;
  iframe.title = label;
  iframe.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share; fullscreen";
  iframe.allowFullscreen = true;
  const root = begin(postId, label, iframe);
  if (root.requestFullscreen && session) {
    session.standardFullscreen = true;
    void root.requestFullscreen().catch(() => {});
  }
}

export function bindExpandedPlayback(postId: string, slot: HTMLElement) {
  const dock = () => {
    if (!session || session.postId !== postId || session.holding) return;
    if (session.media.parentElement !== slot) {
      slot.replaceChildren(session.media);
      styleMediaSlot(session.media);
    }
    session.docked = true;
    if (session.wantsPlay) resumePlayback(session.media);
  };
  session?.subscribers.add(dock);
  dock();
  return () => {
    session?.subscribers.delete(dock);
  };
}

export function retainExpandedPlayback(pathname: string) {
  if (!session) return;
  if (pathname === `/posts/${session.postId}`) return;
  clearSession();
}
