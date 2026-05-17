import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ListRow } from "../lib/api";

interface Props {
  currentSlug?: string;
  currentTitle: string;
}

// ListSwitcher shows the current list title and, on click, drops a panel of
// every active list (grouped by project) so the human can jump straight
// between lists without bouncing through home.
export function ListSwitcher({ currentSlug, currentTitle }: Props) {
  const [open, setOpen] = useState(false);
  const [rows, setRows] = useState<ListRow[] | null>(null);
  const navigate = useNavigate();
  const wrapRef = useRef<HTMLDivElement | null>(null);

  // Lazy-load the list of lists on first open so a fresh ListView doesn't pay
  // the fetch cost before the user asks.
  useEffect(() => {
    if (open && rows === null) {
      api.listLists().then(setRows).catch(() => setRows([]));
    }
  }, [open, rows]);

  // Click-outside + Escape closes the panel.
  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const groups = groupByProject(rows ?? []);
  const totalLists = rows?.length ?? 0;

  const jump = (slug: string) => {
    setOpen(false);
    if (slug !== currentSlug) navigate(`/lists/${slug}`);
  };

  return (
    <div className="list-switcher" ref={wrapRef}>
      <button
        className="list-switcher-trigger"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="list-switcher-title">{currentTitle}</span>
        <Chevron open={open} />
      </button>
      {open && (
        <div className="list-switcher-panel" role="listbox">
          {rows === null && <div className="list-switcher-loading">loading…</div>}
          {rows !== null && totalLists === 0 && (
            <div className="list-switcher-loading">no other lists</div>
          )}
          {groups.map(([proj, items]) => (
            <div key={proj} className="list-switcher-group">
              <div className="list-switcher-group-header">{proj || "(no project)"}</div>
              {items.map((r) => (
                <button
                  key={r.slug}
                  className={`list-switcher-item ${r.slug === currentSlug ? "current" : ""}`}
                  onClick={() => jump(r.slug)}
                  role="option"
                  aria-selected={r.slug === currentSlug}
                >
                  <span className="list-switcher-item-title">{r.title}</span>
                  <span className="list-switcher-item-slug">{r.slug}</span>
                </button>
              ))}
            </div>
          ))}
          <div className="list-switcher-footer">
            <button className="list-switcher-home" onClick={() => { setOpen(false); navigate("/"); }}>
              All lists →
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function groupByProject(rows: ListRow[]): [string, ListRow[]][] {
  const m = new Map<string, ListRow[]>();
  for (const r of rows) {
    const k = r.project_path || "";
    if (!m.has(k)) m.set(k, []);
    m.get(k)!.push(r);
  }
  return [...m.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      style={{
        transform: open ? "rotate(180deg)" : "rotate(0deg)",
        transition: "transform 140ms ease",
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
