import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { api, Item, ListDetail, State } from "../lib/api";
import { useSSE } from "../lib/sse";
import { InputField } from "../components/InputField";
import { StateIcon } from "../components/StateIcon";
import { StateSelect } from "../components/StateSelect";

interface FlatItem {
  item: Item;
  depth: number;
}

function flatten(items: Item[], depth: number, out: FlatItem[]) {
  for (const it of items) {
    out.push({ item: it, depth });
    if (it.children) flatten(it.children, depth + 1, out);
  }
}

export function Popout() {
  const { slug } = useParams<{ slug: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [list, setList] = useState<ListDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const navRef = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    if (!slug) return;
    try {
      const l = await api.getList(slug);
      setList(l);
      setErr(null);
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    }
  }, [slug]);

  useEffect(() => { refresh(); }, [refresh]);

  useEffect(() => {
    if (!list) return;
    document.title = `${list.title} - Meatbag Overlay`;
  }, [list]);

  useSSE(useCallback((ev) => {
    if (!slug) return;
    if ((ev.type === "list_updated" || ev.type === "list_deleted") && ev.slug === slug) {
      refresh();
    }
  }, [refresh, slug]));

  useEffect(() => {
    if (!navOpen) return;
    const handler = (e: MouseEvent) => {
      if (navRef.current && !navRef.current.contains(e.target as Node)) {
        setNavOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [navOpen]);

  const flat: FlatItem[] = [];
  if (list) flatten(list.items, 0, flat);

  const currentItemID = searchParams.get("item");
  const currentIndex = flat.findIndex(f => f.item.id === currentItemID);
  const effectiveIndex = currentIndex >= 0 ? currentIndex : 0;
  const current = flat[effectiveIndex] ?? null;

  const goTo = (index: number) => {
    const f = flat[index];
    if (!f) return;
    setSearchParams({ item: f.item.id }, { replace: true });
    setNavOpen(false);
  };

  const setExact = async (state: State) => {
    if (!slug || !current) return;
    setBusy(true);
    try {
      await api.setState(slug, current.item.id, state);
      await refresh();
    } finally {
      setBusy(false);
    }
  };

  if (err) return <div className="popout-message">{err}</div>;
  if (!list) return <div className="popout-message">Loading…</div>;
  if (flat.length === 0) return <div className="popout-message">No items.</div>;

  const { item } = current!;

  return (
    <div className="popout-root">
      <div className="popout-header" ref={navRef}>
        <button
          className="popout-nav-toggle"
          onClick={() => setNavOpen(v => !v)}
          title="Navigate items"
        >
          <span className="popout-list-title">{list.title}</span>
          <svg className="popout-chevron" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
            <path
              d={navOpen ? "M18 15l-6-6-6 6" : "M6 9l6 6 6-6"}
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>

        {navOpen && (
          <div className="popout-nav-dropdown">
            {flat.map(({ item: it, depth }, i) => (
              <button
                key={it.id}
                className={`popout-nav-item popout-nav-depth-${Math.min(depth, 4)}${i === effectiveIndex ? " active" : ""}`}
                onClick={() => goTo(i)}
              >
                <StateIcon state={it.state} size={12} />
                <span className="popout-nav-label">{it.label}</span>
                <span className="popout-nav-title">{it.title}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="popout-item">
        <div className="popout-item-meta">
          <span className="item-label">{item.label}</span>
          <span className={`owner-tag ${item.owner}`}>{item.owner}</span>
          <span className={`state-text state-${item.state}`}>{item.state.replace("_", " ")}</span>
        </div>
        <h2 className="popout-item-title">{item.title}</h2>
        {item.note && <div className="popout-item-note">{item.note}</div>}
        {item.content_html && (
          <div
            className="markdown popout-markdown"
            dangerouslySetInnerHTML={{ __html: item.content_html }}
          />
        )}
        {item.inputs && item.inputs.length > 0 && (
          <div className="inputs">
            {item.inputs.map(schema => (
              <InputField
                key={schema.name}
                slug={list.slug}
                itemID={item.id}
                schema={schema}
                value={item.input_values?.[schema.name]}
                onChange={refresh}
              />
            ))}
          </div>
        )}
      </div>

      <div className="popout-footer">
        <StateSelect
          current={item.state}
          onPick={setExact}
          disabled={busy}
        />
        <div className="popout-nav-controls">
          <button
            className="popout-step-btn"
            onClick={() => goTo(effectiveIndex - 1)}
            disabled={effectiveIndex === 0}
          >
            ← Prev
          </button>
          <span className="popout-position">{effectiveIndex + 1} / {flat.length}</span>
          <button
            className="popout-step-btn"
            onClick={() => goTo(effectiveIndex + 1)}
            disabled={effectiveIndex === flat.length - 1}
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  );
}
