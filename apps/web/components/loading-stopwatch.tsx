export function LoadingStopwatch() {
  return (
    <svg className="screen-loader-watch" viewBox="0 0 86 96" aria-hidden>
      <rect className="screen-loader-crown" x="37" y="0" width="12" height="7" rx="2" />
      <rect className="screen-loader-crown" x="41" y="6" width="4" height="6" />
      <circle className="screen-loader-face" cx="43" cy="54" r="36" />
      <circle className="screen-loader-arc" cx="43" cy="54" r="32" pathLength="100" />
      <path className="screen-loader-tick" d="M43 24v5M73 54h-5M43 84v-5M13 54h5" />
      <line className="screen-loader-hand" x1="43" y1="54" x2="43" y2="28" />
      <circle className="screen-loader-pin" cx="43" cy="54" r="3.5" />
    </svg>
  );
}
