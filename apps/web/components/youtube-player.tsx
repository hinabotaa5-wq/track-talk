const youtubeOrigin = "https://www.youtube-nocookie.com";

// YouTube のプレーヤーをそのまま置く。前は表紙の画像に TrackTalk の再生ボタンを重ね、押すとプレーヤーに差し替えていたが、
// iPhone などでは自動再生が効かず YouTube の再生ボタンがもう一度出て、2回押すことになっていた。
// 再生ボタンは YouTube のものだけにして、1回で再生できるようにする。
export function YouTubePlayer({ videoId, title }: { videoId: string; title: string }) {
  return (
    <iframe
      className="aspect-video w-full rounded-xl bg-black shadow-sm"
      src={`${youtubeOrigin}/embed/${videoId}?playsinline=1&rel=0`}
      title={title}
      loading="lazy"
      allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
      allowFullScreen
      referrerPolicy="strict-origin-when-cross-origin"
    />
  );
}
