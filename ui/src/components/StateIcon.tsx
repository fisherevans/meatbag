import { State } from "../lib/api";

interface Props {
  state: State;
  size?: number;
  className?: string;
}

// StateIcon renders a 24-grid SVG icon for the given state, color-coded via
// currentColor (which the surrounding CSS sets per state class).
export function StateIcon({ state, size = 22, className = "" }: Props) {
  const cls = `state-icon state-icon-${state} ${className}`.trim();
  const common = {
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    className: cls,
    "aria-label": state,
  } as const;

  switch (state) {
    case "done":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="11" fill="currentColor" />
          <path
            d="M7 12.5l3.2 3.2L17 9"
            stroke="white"
            strokeWidth="2.4"
            fill="none"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      );
    case "in_progress":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="10.5" fill="none" stroke="currentColor" strokeWidth="2" />
          <path
            d="M12 1.5a10.5 10.5 0 0 1 0 21z"
            fill="currentColor"
          />
        </svg>
      );
    case "blocked":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="11" fill="currentColor" />
          <rect x="10.7" y="6.5" width="2.6" height="7.5" rx="1.3" fill="white" />
          <circle cx="12" cy="17" r="1.5" fill="white" />
        </svg>
      );
    case "skipped":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="10.5" fill="none" stroke="currentColor" strokeWidth="2" />
          <line x1="6" y1="18" x2="18" y2="6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
      );
    default: // todo
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="10.5" fill="none" stroke="currentColor" strokeWidth="2" />
        </svg>
      );
  }
}
