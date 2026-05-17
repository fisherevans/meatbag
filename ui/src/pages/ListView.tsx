import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, ListDetail } from "../lib/api";
import { useSSE } from "../lib/sse";
import { ItemNode } from "../components/ItemNode";
import { Sidebar } from "../components/Sidebar";
import { BrandMark } from "../components/BrandMark";
import { ListSwitcher } from "../components/ListSwitcher";

export function ListView() {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const [list, setList] = useState<ListDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [activeID, setActiveID] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // IntersectionObserver: pick the item closest to the top of the viewport as
  // "active" so the sidebar can highlight it.
  const observerRef = useRef<IntersectionObserver | null>(null);
  const elementsRef = useRef<Map<string, HTMLElement>>(new Map());

  useEffect(() => {
    observerRef.current = new IntersectionObserver(
      () => {
        // Pick topmost element whose top is below the viewport's top edge but
        // closest to it.
        let bestID: string | null = null;
        let bestTop = -Infinity;
        elementsRef.current.forEach((el, id) => {
          const rect = el.getBoundingClientRect();
          // We want the item whose top is closest to (but not far below) the
          // top of the viewport.
          if (rect.top <= 120 && rect.top > bestTop) {
            bestTop = rect.top;
            bestID = id;
          }
        });
        setActiveID(bestID);
      },
      { rootMargin: "-100px 0px -60% 0px", threshold: [0, 0.2, 0.5, 1] }
    );
    return () => observerRef.current?.disconnect();
  }, []);

  const registerObserver = useCallback((id: string, el: HTMLElement | null) => {
    const obs = observerRef.current;
    const prev = elementsRef.current.get(id);
    if (prev && obs) obs.unobserve(prev);
    if (el) {
      elementsRef.current.set(id, el);
      obs?.observe(el);
    } else {
      elementsRef.current.delete(id);
    }
  }, []);

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

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    const name = list?.title ?? slug;
    document.title = name ? `${name} - Meatbag` : "Meatbag";
  }, [list, slug]);

  useSSE(
    useCallback(
      (ev) => {
        if (!slug) return;
        if ((ev.type === "list_updated" || ev.type === "list_deleted") && ev.slug === slug) {
          refresh();
        }
      },
      [refresh, slug]
    )
  );

  // Scroll to the URL fragment item exactly once, the first time the list
  // becomes available. This handles the genuine fragment-navigation cases:
  // (a) initial page load with a hash in the URL bar, and (b) deep links
  // pasted from `meatbag url`. Sidebar clicks manage their own scrolling and
  // must not trigger this effect, otherwise unrelated re-renders (e.g. an
  // SSE-driven refetch after a checkbox toggle) would re-scroll back to the
  // sidebar's fragment target.
  const didInitialScrollRef = useRef(false);
  useEffect(() => {
    if (!list || didInitialScrollRef.current) return;
    didInitialScrollRef.current = true;
    const hash = window.location.hash.slice(1);
    if (!hash) return;
    const el = document.getElementById(hash);
    if (!el) return;
    el.scrollIntoView({ behavior: "smooth", block: "start" });
    el.classList.add("highlight");
    const t = setTimeout(() => el.classList.remove("highlight"), 2400);
    return () => clearTimeout(t);
  }, [list]);

  // Re-run the fragment scroll on actual hash navigation (browser back/forward
  // to a fragment URL). Sidebar clicks use history.replaceState which does NOT
  // fire hashchange, so this stays scoped to genuine navigation events.
  useEffect(() => {
    const onHashChange = () => {
      const hash = window.location.hash.slice(1);
      if (!hash) return;
      const el = document.getElementById(hash);
      if (!el) return;
      el.scrollIntoView({ behavior: "smooth", block: "start" });
      el.classList.add("highlight");
      setTimeout(() => el.classList.remove("highlight"), 2400);
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const doArchive = async () => {
    if (!slug) return;
    setBusy(true);
    try {
      await api.archiveList(slug);
      await refresh();
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const doRestore = async () => {
    if (!slug) return;
    setBusy(true);
    try {
      await api.unarchiveList(slug);
      await refresh();
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const doDelete = async () => {
    if (!slug || !list) return;
    const ok = window.confirm(
      `Delete "${list.title}" permanently?\n\nThis purges any stored passwords and uploaded files referenced by the list. There is no undo.`
    );
    if (!ok) return;
    setBusy(true);
    try {
      await api.deleteList(slug);
      navigate("/");
    } catch (e: any) {
      setErr(String(e?.message ?? e));
      setBusy(false);
    }
  };

  const archived = list?.status === "archived";

  return (
    <div className="page list-page">
      <header className="top">
        <Link to="/" className="brand" aria-label="home">
          <BrandMark />
          <span className="brand-text">meatbag</span>
        </Link>
        <span className="top-divider" aria-hidden />
        <ListSwitcher currentSlug={list?.slug ?? slug} currentTitle={list ? list.title : slug ?? "…"} />
        {list && archived && <span className="status-badge archived">archived</span>}
        <div className="top-spacer" />
        {list && (
          <div className="top-actions">
            {archived ? (
              <button
                className="btn btn-ghost btn-small"
                disabled={busy}
                onClick={doRestore}
                title="Restore list to active"
              >
                Restore
              </button>
            ) : (
              <button
                className="btn btn-ghost btn-small"
                disabled={busy}
                onClick={doArchive}
                title="Move list to archive"
              >
                Archive
              </button>
            )}
            <button
              className="btn btn-danger btn-small"
              disabled={busy}
              onClick={doDelete}
              title="Delete list and purge secrets/blobs"
            >
              Delete
            </button>
          </div>
        )}
      </header>
      {err && <div className="empty">{err}</div>}
      {list && (
        <div className="list-layout">
          <aside className="sidebar-wrap">
            <Sidebar items={list.items} activeID={activeID} />
          </aside>
          <main className="list-main">
            {list.description_html && (
              <div
                className="list-desc markdown"
                dangerouslySetInnerHTML={{ __html: list.description_html }}
              />
            )}
            <div className="tree">
              {list.items.length === 0 && <div className="empty">No items yet.</div>}
              {list.items.map((it) => (
                <ItemNode
                  key={it.id}
                  slug={list.slug}
                  item={it}
                  depth={0}
                  onChange={refresh}
                  registerObserver={registerObserver}
                />
              ))}
            </div>
          </main>
        </div>
      )}
    </div>
  );
}
