export type State = "todo" | "in_progress" | "blocked" | "done" | "skipped";
export type Owner = "human" | "agent";

export type InputType =
  | "text"
  | "textarea"
  | "password"
  | "number"
  | "url"
  | "select"
  | "multiselect"
  | "radio"
  | "checkbox"
  | "file"
  | "markdown";

export interface InputSchema {
  name: string;
  type: InputType;
  label?: string;
  description?: string;
  required?: boolean;
  options?: string[];
  accept?: string[];
  default?: unknown;
}

export interface InputValue {
  value?: unknown;
  secret_ref?: string;
  blob_ref?: string;
  filename?: string;
  size?: number;
  has_value: boolean;
}

export interface Item {
  id: string;
  label: string;
  title: string;
  owner: Owner;
  state: State;
  content?: string;
  content_html?: string;
  inputs?: InputSchema[];
  input_values?: Record<string, InputValue>;
  note?: string;
  children?: Item[];
}

export interface ListRow {
  id: string;
  slug: string;
  title: string;
  project_path?: string;
  status: string;
  updated_at: string;
  progress: {
    todo: number;
    in_progress: number;
    blocked: number;
    done: number;
    skipped: number;
    awaiting_input: number;
  };
}

export interface ListDetail {
  id: string;
  slug: string;
  title: string;
  description?: string;
  description_html?: string;
  project_path?: string;
  status: string;
  updated_at: string;
  items: Item[];
}

async function jget<T>(url: string): Promise<T> {
  const r = await fetch(url);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

async function jsend<T>(url: string, method: string, body?: unknown): Promise<T> {
  const r = await fetch(url, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export const api = {
  listLists: () => jget<ListRow[]>("/api/lists?status=active"),
  listListsArchived: () => jget<ListRow[]>("/api/lists?status=archived"),
  getList: (slug: string) => jget<ListDetail>(`/api/lists/${slug}`),
  setState: (slug: string, itemID: string, state: State, note?: string) =>
    jsend(`/api/lists/${slug}/items/${itemID}/state`, "POST", { state, note }),
  setInput: (slug: string, itemID: string, field: string, value: unknown) =>
    jsend(`/api/lists/${slug}/items/${itemID}/inputs/${field}`, "POST", { value }),
  uploadFile: async (slug: string, itemID: string, field: string, file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    const r = await fetch(`/api/lists/${slug}/items/${itemID}/inputs/${field}`, {
      method: "POST",
      body: fd,
    });
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
  clearInput: (slug: string, itemID: string, field: string) =>
    jsend(`/api/lists/${slug}/items/${itemID}/inputs/${field}`, "DELETE"),
};
