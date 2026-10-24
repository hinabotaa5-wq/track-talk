const youtubeOrigin = "https://www.youtube-nocookie.com";

// YouTube のプレーヤーをそのまま置く。前は表紙の画像に TrackTalk の再生ボタンを重ね、押すとプレーヤーに差し替えていたが、
// iPhone などでは自動再生が効かず YouTube の再生ボタンがもう一度出て、2回押すことになっていた。
// 再生ボタンは YouTube のものだけにして、1回で再生できるようにする。
export function YouTubePlayer({ videoId, title, shorts }: { videoId: string; title: string; shorts?: boolean }) {
  return (
    <iframe
      className={
        shorts
          ? "mx-auto aspect-[9/16] max-h-[50dvh] w-full max-w-[28.125dvh] rounded-xl bg-black shadow-sm"
          : "mx-auto aspect-video w-full max-w-[560px] rounded-xl bg-black shadow-sm"
      }
      src={`${youtubeOrigin}/embed/${videoId}?playsinline=1&rel=0&cc_load_policy=0`}
      title={title}
      loading="lazy"
      allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
      allowFullScreen
      referrerPolicy="strict-origin-when-cross-origin"
    />
  );
}
