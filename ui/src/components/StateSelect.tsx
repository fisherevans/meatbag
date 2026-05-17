import { useEffect, useRef, useState } from "react";
import { State } from "../lib/api";
import { StateIcon } from "./StateIcon";

const STATES: State[] = ["todo", "in_progress", "blocked", "done", "skipped"];

interface Props {
  current: State;
  onPick: (s: State) => void;
  disabled?: boolean;
}

// StateSelect is the always-visible status dropdown in an item's action area.
// Click the trigger to open a themed panel of all states; click an entry to
// pick it. Outside click or Escape closes. This is distinct from StatePicker,
// which is the popover that appears on long-press of the state icon.
export function StateSelect({ current, onPick, disabled }: Props) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent | TouchEvent) => {
      if (!wrapRef.current) return;
      if (wrapRef.current.contains(e.target as Node)) return;
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("touchstart", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("touchstart", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const pick = (s: State) => {
    setOpen(false);
    if (s !== current) onPick(s);
  };

  return (
    <div className="state-select-wrap" ref={wrapRef}>
      <button
        type="button"
        className={`state-select-trigger state-${current}`}
        onClick={() => setOpen((o) => !o)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={`status: ${prettyState(current)}, click to change`}
      >
        <StateIcon state={current} size={16} />
        <span className="state-select-label">{prettyState(current)}</span>
        <Chevron open={open} />
      </button>
      {open && (
        <div className="state-select-panel" role="listbox">
          {STATES.map((s) => (
            <button
              key={s}
              type="button"
              role="option"
              aria-selected={s === current}
              className={`state-select-item state-${s} ${s === current ? "current" : ""}`}
              onClick={() => pick(s)}
            >
              <StateIcon state={s} size={18} />
              <span className="state-select-item-label">{prettyState(s)}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function prettyState(s: State): string {
  return s.replace("_", " ");
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      aria-hidden="true"
      style={{
        transform: open ? "rotate(180deg)" : "rotate(0deg)",
        transition: "transform 140ms ease",
        flexShrink: 0,
      }}
    >
      <path
        d="M6 9l6 6 6-6"
        stroke="currentColor"
        strokeWidth="2"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
