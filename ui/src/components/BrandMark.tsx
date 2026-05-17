// BrandMark is the small icon paired with the "meatbag" wordmark in the top
// header. Placeholder shape until a real logo lands.
export function BrandMark({ size = 22 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className="brand-mark"
      aria-hidden
    >
      <defs>
        <linearGradient id="bm-g" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#6aa7ff" />
          <stop offset="1" stopColor="#3d6cd0" />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="30" height="30" rx="8" fill="url(#bm-g)" />
      <path
        d="M9 16.5l4.5 4.5L23 11"
        stroke="#0c0e13"
        strokeWidth="3"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
