import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, ListRow } from "../lib/api";
import { useSSE } from "../lib/sse";
import { BrandMark } from "../components/BrandMark";

type Filter = "active" | "archived";

export function Home() {
  const [filter, setFilter] = useState<Filter>("active");
  const [rows, setRows] = useState<ListRow[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [busySlug, setBusySlug] = useState<string | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    document.title = "Meatbag";
  }, []);

  const refresh = useCallback(async () => {
    try {
      const r = filter === "archived"
        ? await api.listListsArchived()
        : await api.listLists();
      setRows(r);
      setErr(null);
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    }
  }, [filter]);

  useEffect(() => {
    // Clear stale rows when switching filters so the new list pops in fresh
    // rather than briefly showing the wrong status.
    setRows(null);
    refresh();
  }, [refresh]);

  useSSE(useCallback((ev) => {
    if (ev.type === "list_updated" || ev.type === "list_deleted") refresh();
  }, [refresh]));

  const doArchive = async (slug: string) => {
    setBusySlug(slug);
    try {
      await api.archiveList(slug);
      await refresh();
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    } finally {
      setBusySlug(null);
    }
  };

  const doRestore = async (slug: string) => {
    setBusySlug(slug);
    try {
      await api.unarchiveList(slug);
      await refresh();
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    } finally {
      setBusySlug(null);
    }
  };

  const doDelete = async (slug: string, title: string) => {
    const ok = window.confirm(
      `Delete "${title}" permanently?\n\nThis purges any stored passwords and uploaded files referenced by the list. There is no undo.`
    );
    if (!ok) return;
    setBusySlug(slug);
    try {
      await api.deleteList(slug);
      await refresh();
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    } finally {
      setBusySlug(null);
    }
  };

  return (
    <div className="page">
      <header className="top">
        <Link to="/" className="brand" aria-label="home">
          <BrandMark />
          <span className="brand-text">meatbag</span>
        </Link>
        <span className="top-divider" aria-hidden />
        <h2 className="top-title">All lists</h2>
        <div className="top-spacer" />
        <div className="status-toggle" role="tablist" aria-label="list status">
          <button
            role="tab"
            aria-selected={filter === "active"}
            className={`status-toggle-btn ${filter === "active" ? "active" : ""}`}
            onClick={() => setFilter("active")}
          >
            Active
          </button>
          <button
            role="tab"
            aria-selected={filter === "archived"}
            className={`status-toggle-btn ${filter === "archived" ? "active" : ""}`}
            onClick={() => setFilter("archived")}
          >
            Archived
          </button>
        </div>
      </header>
      <div className="home-main">
        {err && <div className="empty">{err}</div>}
        {rows && rows.length === 0 && filter === "active" && (
          <div className="empty">
            No lists yet. Create one with{" "}
            <code>meatbag list create --title "..."</code>.
          </div>
        )}
        {rows && rows.length === 0 && filter === "archived" && (
          <div className="empty">No archived lists.</div>
        )}
        {rows && groupByProject(rows).map(([proj, group]) => (
          <section key={proj}>
            <div className="group-header">{proj || "(no project)"}</div>
            {group.map((r) => (
              <div
                key={r.slug}
                className="list-row"
                role="link"
                tabIndex={0}
                onClick={() => navigate(`/lists/${r.slug}`)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    navigate(`/lists/${r.slug}`);
                  }
                }}
              >
                <div>
                  <div className="title">
                    {r.title}
                    <span className="slug">{r.slug}</span>
                  </div>
                  <div className="meta">updated {fmtDate(r.updated_at)}</div>
                </div>
                <div className="list-row-right">
                  <div className="progress">
                    {totalDone(r)} / {totalItems(r)} done
                    {r.progress.in_progress > 0 && <> · {r.progress.in_progress} in progress</>}
                    {r.progress.blocked > 0 && <> · {r.progress.blocked} blocked</>}
                    {r.progress.awaiting_input > 0 && (
                      <div className="awaiting">{r.progress.awaiting_input} awaiting input</div>
                    )}
                  </div>
                  <div className="row-actions" onClick={(e) => e.stopPropagation()}>
                    {filter === "active" ? (
                      <button
                        className="btn btn-ghost btn-small"
                        disabled={busySlug === r.slug}
                        onClick={() => doArchive(r.slug)}
                        title="Move list to archive"
                      >
                        Archive
                      </button>
                    ) : (
                      <button
                        className="btn btn-ghost btn-small"
                        disabled={busySlug === r.slug}
                        onClick={() => doRestore(r.slug)}
                        title="Restore list to active"
                      >
                        Restore
                      </button>
                    )}
                    <button
                      className="btn btn-danger btn-small"
                      disabled={busySlug === r.slug}
                      onClick={() => doDelete(r.slug, r.title)}
                      title="Delete list and purge secrets/blobs"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </section>
        ))}
      </div>
    </div>
  );
}

function groupByProject(rows: ListRow[]): [string, ListRow[]][] {
  const groups = new Map<string, ListRow[]>();
  for (const r of rows) {
    const k = r.project_path || "";
    if (!groups.has(k)) groups.set(k, []);
    groups.get(k)!.push(r);
  }
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function totalDone(r: ListRow): number {
  return r.progress.done + r.progress.skipped;
}
function totalItems(r: ListRow): number {
  const p = r.progress;
  return p.todo + p.in_progress + p.blocked + p.done + p.skipped;
}

function fmtDate(s: string): string {
  try {
    const d = new Date(s);
    return d.toLocaleString();
  } catch {
    return s;
  }
}
