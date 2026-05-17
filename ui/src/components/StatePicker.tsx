import { useEffect, useRef } from "react";
import { State } from "../lib/api";
import { StateIcon } from "./StateIcon";

const STATES: State[] = ["todo", "in_progress", "blocked", "done", "skipped"];

interface Props {
  current: State;
  onPick: (s: State) => void;
  onDismiss: () => void;
}

// StatePicker renders a small popover listing every state. Closes on outside
// click or Escape. Positioning is handled by the parent via CSS - this
// component just renders the panel.
export function StatePicker({ current, onPick, onDismiss }: Props) {
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onDocDown = (e: MouseEvent | TouchEvent) => {
      if (!ref.current) return;
      if (ref.current.contains(e.target as Node)) return;
      onDismiss();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onDismiss();
    };
    // Defer attaching so the same event that opened the picker doesn't close it.
    const t = window.setTimeout(() => {
      document.addEventListener("mousedown", onDocDown);
      document.addEventListener("touchstart", onDocDown);
      document.addEventListener("keydown", onKey);
    }, 0);
    return () => {
      window.clearTimeout(t);
      document.removeEventListener("mousedown", onDocDown);
      document.removeEventListener("touchstart", onDocDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [onDismiss]);

  return (
    <div ref={ref} className="state-picker" role="menu" aria-label="set status">
      {STATES.map((s) => (
        <button
          key={s}
          role="menuitemradio"
          aria-checked={s === current}
          className={`state-picker-item state-${s} ${s === current ? "current" : ""}`}
          onClick={() => onPick(s)}
        >
          <StateIcon state={s} size={20} />
          <span className="state-picker-label">{prettyState(s)}</span>
        </button>
      ))}
    </div>
  );
}

function prettyState(s: State): string {
  return s.replace("_", " ");
}
