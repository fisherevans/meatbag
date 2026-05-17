import { useState } from "react";
import { api, InputSchema, InputValue } from "../lib/api";

type Status = "idle" | "saving" | "saved" | "error";

interface Props {
  slug: string;
  itemID: string;
  schema: InputSchema;
  value?: InputValue;
  onChange: () => void;
}

export function InputField({ slug, itemID, schema, value, onChange }: Props) {
  const [draft, setDraft] = useState<string>(() =>
    value?.value != null && schema.type !== "password" ? String(value.value) : ""
  );
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState<string | null>(null);
  const hasValue = !!value?.has_value;
  const fieldID = `input-${itemID}-${schema.name}`;

  const flashSaved = () => {
    setStatus("saved");
    setError(null);
    window.setTimeout(() => setStatus("idle"), 1500);
  };

  const flashError = (msg: string) => {
    setStatus("error");
    setError(msg);
  };

  const wrap = async (fn: () => Promise<unknown>) => {
    setStatus("saving");
    setError(null);
    try {
      await fn();
      onChange();
      flashSaved();
    } catch (e: any) {
      flashError(String(e?.message ?? e));
    }
  };

  const save = (v: unknown) => wrap(() => api.setInput(slug, itemID, schema.name, v));

  const clear = async () => {
    if (!confirm(`Clear ${schema.name}?`)) return;
    await wrap(() => api.clearInput(slug, itemID, schema.name));
    setDraft("");
  };

  const onUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    await wrap(() => api.uploadFile(slug, itemID, schema.name, file));
    e.target.value = "";
  };

  return (
    <div id={fieldID} className={`input-group input-type-${schema.type} status-${status}`}>
      <div className="input-header">
        <label className="input-label" htmlFor={`${fieldID}-control`}>
          {schema.label || schema.name}
          {schema.required && <span className="req" title="required">*</span>}
        </label>
        <span className="input-meta">
          {hasValue && status !== "saved" && <span className="set-badge">filled</span>}
          {status === "saving" && <span className="status-pill saving">saving…</span>}
          {status === "saved" && <span className="status-pill saved">saved ✓</span>}
          {status === "error" && <span className="status-pill error">error</span>}
        </span>
      </div>
      {schema.description && <div className="input-desc">{schema.description}</div>}
      <div className="input-body">{renderControl()}</div>
      {error && <div className="input-error">{error}</div>}
    </div>
  );

  function renderControl() {
    const id = `${fieldID}-control`;
    const busy = status === "saving";

    switch (schema.type) {
      case "password":
        return (
          <div className="input-row">
            <input
              id={id}
              type="password"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={hasValue ? "Enter a new value to replace" : "Enter value"}
              disabled={busy}
              className="text-input"
            />
            <button
              className="btn btn-primary btn-large"
              disabled={busy || !draft}
              onClick={() => save(draft).then(() => setDraft(""))}
            >
              {hasValue ? "Replace" : "Save"}
            </button>
            {hasValue && (
              <button className="btn btn-ghost" disabled={busy} onClick={clear}>
                Clear
              </button>
            )}
          </div>
        );

      case "textarea":
      case "markdown":
        return (
          <>
            <textarea
              id={id}
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              disabled={busy}
              className="text-input"
              rows={5}
            />
            <div className="input-row">
              <button className="btn btn-primary btn-large" disabled={busy} onClick={() => save(draft)}>
                Save
              </button>
              {hasValue && (
                <button className="btn btn-ghost" disabled={busy} onClick={clear}>
                  Clear
                </button>
              )}
            </div>
          </>
        );

      case "number":
        return (
          <div className="input-row">
            <input
              id={id}
              type="number"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              disabled={busy}
              className="text-input"
            />
            <button className="btn btn-primary btn-large" disabled={busy} onClick={() => save(Number(draft))}>
              Save
            </button>
            {hasValue && (
              <button className="btn btn-ghost" disabled={busy} onClick={clear}>
                Clear
              </button>
            )}
          </div>
        );

      case "checkbox":
        return (
          <div className="input-row">
            <label className="toggle">
              <input
                id={id}
                type="checkbox"
                checked={value?.value === true}
                onChange={(e) => save(e.target.checked)}
                disabled={busy}
              />
              <span>{value?.value === true ? "on" : "off"}</span>
            </label>
          </div>
        );

      case "select":
      case "radio": {
        const opts = schema.options ?? [];
        const current = typeof value?.value === "string" ? value!.value : draft;
        return (
          <ul className="option-list">
            {opts.map((o) => (
              <li key={o}>
                <label className={`option-row ${current === o ? "selected" : ""}`}>
                  <input
                    type="radio"
                    name={`${fieldID}-radio`}
                    checked={current === o}
                    onChange={() => {
                      setDraft(o);
                      save(o);
                    }}
                    disabled={busy}
                  />
                  <span>{o}</span>
                </label>
              </li>
            ))}
            {hasValue && (
              <li>
                <button className="btn btn-ghost btn-small" disabled={busy} onClick={clear}>
                  Clear selection
                </button>
              </li>
            )}
          </ul>
        );
      }

      case "multiselect": {
        const opts = schema.options ?? [];
        const current: string[] = Array.isArray(value?.value) ? (value!.value as string[]) : [];
        const toggle = (o: string) => {
          const next = current.includes(o) ? current.filter((x) => x !== o) : [...current, o];
          save(next);
        };
        return (
          <ul className="option-list">
            {opts.map((o) => (
              <li key={o}>
                <label className={`option-row ${current.includes(o) ? "selected" : ""}`}>
                  <input
                    type="checkbox"
                    checked={current.includes(o)}
                    onChange={() => toggle(o)}
                    disabled={busy}
                  />
                  <span>{o}</span>
                </label>
              </li>
            ))}
          </ul>
        );
      }

      case "file":
        return (
          <div className="input-row file-row">
            <label className="btn btn-primary btn-large file-btn">
              {hasValue ? "Replace file" : "Choose file"}
              <input
                id={id}
                type="file"
                onChange={onUpload}
                disabled={busy}
                accept={(schema.accept ?? []).join(",")}
                style={{ display: "none" }}
              />
            </label>
            {hasValue && (
              <>
                <span className="file-meta">
                  {value?.filename || "uploaded"} · {humanSize(value?.size ?? 0)}
                </span>
                <button className="btn btn-ghost" disabled={busy} onClick={clear}>
                  Clear
                </button>
              </>
            )}
          </div>
        );

      case "url":
        return (
          <div className="input-row">
            <input
              id={id}
              type="url"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              disabled={busy}
              className="text-input"
            />
            <button className="btn btn-primary btn-large" disabled={busy} onClick={() => save(draft)}>
              Save
            </button>
            {hasValue && (
              <button className="btn btn-ghost" disabled={busy} onClick={clear}>
                Clear
              </button>
            )}
          </div>
        );

      default: // text
        return (
          <div className="input-row">
            <input
              id={id}
              type="text"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              disabled={busy}
              className="text-input"
            />
            <button className="btn btn-primary btn-large" disabled={busy} onClick={() => save(draft)}>
              Save
            </button>
            {hasValue && (
              <button className="btn btn-ghost" disabled={busy} onClick={clear}>
                Clear
              </button>
            )}
          </div>
        );
    }
  }
}

function humanSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
