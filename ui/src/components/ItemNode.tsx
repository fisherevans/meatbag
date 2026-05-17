import { useEffect, useRef, useState } from "react";
import { api, Item, State } from "../lib/api";
import { InputField } from "./InputField";
import { StateIcon } from "./StateIcon";

const STATES: State[] = ["todo", "in_progress", "blocked", "done", "skipped"];

interface Props {
  slug: string;
  item: Item;
  depth: number;
  onChange: () => void;
  registerObserver: (id: string, el: HTMLElement | null) => void;
}

export function ItemNode({ slug, item, depth, onChange, registerObserver }: Props) {
  // Default-collapse done / skipped items so attention defaults to open work.
  const [collapsed, setCollapsed] = useState<boolean>(
    item.state === "done" || item.state === "skipped"
  );
  const [busy, setBusy] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    registerObserver(item.id, ref.current);
    return () => registerObserver(item.id, null);
  }, [item.id, registerObserver]);

  const setExact = async (s: State) => {
    setBusy(true);
    try {
      await api.setState(slug, item.id, s);
      onChange();
    } finally {
      setBusy(false);
    }
  };

  const cycle = () => {
    const i = STATES.indexOf(item.state);
    setExact(STATES[(i + 1) % STATES.length]);
  };

  const isDone = item.state === "done";
  const isSkipped = item.state === "skipped";
  const isOpen = !isDone && !isSkipped;

  const hasBody =
    !!item.content_html ||
    (item.inputs && item.inputs.length > 0) ||
    (item.children && item.children.length > 0);

  return (
    <article
      id={`item-${item.id}`}
      ref={ref}
      className={`item depth-${Math.min(depth, 4)} state-${item.state} ${
        collapsed ? "collapsed" : "expanded"
      } ${isDone ? "is-done" : ""}`}
    >
      <header className="item-header">
        <button
          className="state-toggle"
          onClick={cycle}
          disabled={busy}
          title="Click to cycle status"
          aria-label={`status: ${item.state}, click to cycle`}
        >
          <StateIcon state={item.state} size={28} />
        </button>

        <div className="item-meta">
          <div className="item-meta-row">
            <span className="item-label">{item.label}</span>
            <span className={`owner-tag ${item.owner}`}>{item.owner}</span>
            <span className={`state-text state-${item.state}`}>
              {prettyState(item.state)}
            </span>
          </div>
          <h3 className="item-title">{item.title}</h3>
          {item.note && <div className="item-note">{item.note}</div>}
        </div>

        <div className="item-actions">
          {isOpen && (
            <button
              className="btn btn-done btn-large"
              onClick={() => setExact("done")}
              disabled={busy}
            >
              Mark done
            </button>
          )}
          {isDone && (
            <button
              className="btn btn-ghost btn-large"
              onClick={() => setExact("todo")}
              disabled={busy}
            >
              Reopen
            </button>
          )}
          <select
            className="state-select"
            value={item.state}
            onChange={(e) => setExact(e.target.value as State)}
            disabled={busy}
            aria-label="set status"
          >
            {STATES.map((s) => (
              <option key={s} value={s}>{prettyState(s)}</option>
            ))}
          </select>
          {hasBody && (
            <button
              className="collapse-btn"
              onClick={() => setCollapsed((c) => !c)}
              aria-label={collapsed ? "expand" : "collapse"}
              title={collapsed ? "Expand" : "Collapse"}
            >
              <Chevron open={!collapsed} />
            </button>
          )}
        </div>
      </header>

      {!collapsed && (
        <div className="item-body">
          {item.content_html && (
            <div
              className="markdown"
              dangerouslySetInnerHTML={{ __html: item.content_html }}
            />
          )}
          {item.inputs && item.inputs.length > 0 && (
            <div className="inputs">
              {item.inputs.map((schema) => (
                <InputField
                  key={schema.name}
                  slug={slug}
                  itemID={item.id}
                  schema={schema}
                  value={item.input_values?.[schema.name]}
                  onChange={onChange}
                />
              ))}
            </div>
          )}
          {item.children && item.children.length > 0 && (
            <div className="children">
              {item.children.map((c) => (
                <ItemNode
                  key={c.id}
                  slug={slug}
                  item={c}
                  depth={depth + 1}
                  onChange={onChange}
                  registerObserver={registerObserver}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </article>
  );
}

function prettyState(s: State): string {
  return s.replace("_", " ");
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      style={{ transform: open ? "rotate(90deg)" : "rotate(0deg)", transition: "transform 120ms ease" }}
    >
      <path
        d="M9 6l6 6-6 6"
        stroke="currentColor"
        strokeWidth="2"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
