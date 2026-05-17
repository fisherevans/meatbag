import { useState } from "react";
import { Item } from "../lib/api";
import { StateIcon } from "./StateIcon";

interface Flat {
  id: string;
  label: string;
  title: string;
  depth: number;
  state: Item["state"];
  owner: Item["owner"];
}

function flatten(items: Item[], depth: number, out: Flat[]) {
  for (const it of items) {
    out.push({ id: it.id, label: it.label, title: it.title, depth, state: it.state, owner: it.owner });
    if (it.children) flatten(it.children, depth + 1, out);
  }
}

interface Props {
  items: Item[];
  activeID: string | null;
}

export function Sidebar({ items, activeID }: Props) {
  const flat: Flat[] = [];
  flatten(items, 0, flat);

  // flashID is the sidebar row + target item we briefly highlight after a
  // click, so the user can confirm where they jumped to.
  const [flashID, setFlashID] = useState<string | null>(null);

  const onClick = (id: string) => {
    const el = document.getElementById(`item-${id}`);
    if (!el) return;
    el.scrollIntoView({ behavior: "smooth", block: "start" });
    history.replaceState(null, "", `#item-${id}`);
    // Flash the matching item card.
    el.classList.remove("flash"); // restart animation if user re-clicks
    // Force a reflow so removing + re-adding actually replays the keyframes.
    void el.offsetWidth;
    el.classList.add("flash");
    window.setTimeout(() => el.classList.remove("flash"), 1600);
    // Flash the sidebar row.
    setFlashID(id);
    window.setTimeout(() => setFlashID((cur) => (cur === id ? null : cur)), 1600);
  };

  return (
    <nav className="sidebar">
      <div className="sidebar-header">Contents</div>
      <ol className="sidebar-list">
        {flat.map((it) => {
          const isActive = activeID === it.id;
          const isFlash = flashID === it.id;
          return (
            <li
              key={it.id}
              className={`sidebar-item depth-${Math.min(it.depth, 4)} state-${it.state}` +
                (isActive ? " active" : "") +
                (isFlash ? " flash" : "")}
              onClick={() => onClick(it.id)}
            >
              <span className={`state-color state-${it.state}`}>
                <StateIcon state={it.state} size={14} />
              </span>
              <span className="sidebar-label">{it.label}</span>
              <span className="sidebar-title">{it.title}</span>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
