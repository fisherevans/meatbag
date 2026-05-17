import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, ListRow } from "../lib/api";
import { useSSE } from "../lib/sse";
import { BrandMark } from "../components/BrandMark";

export function Home() {
  const [rows, setRows] = useState<ListRow[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const r = await api.listLists();
      setRows(r);
      setErr(null);
    } catch (e: any) {
      setErr(String(e?.message ?? e));
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useSSE(useCallback((ev) => {
    if (ev.type === "list_updated" || ev.type === "list_deleted") refresh();
  }, [refresh]));

  return (
    <div className="page">
      <header className="top">
        <Link to="/" className="brand" aria-label="home">
          <BrandMark size={22} />
          <span className="brand-text">meatbag</span>
        </Link>
        <span className="top-divider" aria-hidden />
        <h2 className="top-title">All lists</h2>
      </header>
      <div className="home-main">
        {err && <div className="empty">{err}</div>}
        {rows && rows.length === 0 && (
          <div className="empty">
            No lists yet. Create one with{" "}
            <code>meatbag list create --title "..."</code>.
          </div>
        )}
        {rows && groupByProject(rows).map(([proj, group]) => (
          <section key={proj}>
            <div className="group-header">{proj || "(no project)"}</div>
            {group.map((r) => (
              <Link to={`/lists/${r.slug}`} key={r.slug} className="list-row">
                <div>
                  <div className="title">
                    {r.title}
                    <span className="slug">{r.slug}</span>
                  </div>
                  <div className="meta">updated {fmtDate(r.updated_at)}</div>
                </div>
                <div className="progress">
                  {totalDone(r)} / {totalItems(r)} done
                  {r.progress.in_progress > 0 && <> · {r.progress.in_progress} in progress</>}
                  {r.progress.blocked > 0 && <> · {r.progress.blocked} blocked</>}
                  {r.progress.awaiting_input > 0 && (
                    <div className="awaiting">{r.progress.awaiting_input} awaiting input</div>
                  )}
                </div>
              </Link>
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
