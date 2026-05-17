import { useEffect } from "react";

type Handler = (ev: { type: string; slug?: string }) => void;

// useSSE opens a single EventSource for the page and invokes `onEvent` for
// each meatbag event. Auto-reconnects after a 1s backoff if the connection
// drops.
export function useSSE(onEvent: Handler) {
  useEffect(() => {
    let stopped = false;
    let es: EventSource | null = null;
    let retryTimer: number | undefined;

    const open = () => {
      if (stopped) return;
      es = new EventSource("/api/events");
      const handle = (e: MessageEvent) => {
        try {
          const data = e.data ? JSON.parse(e.data) : {};
          onEvent({ type: e.type, ...data });
        } catch {
          // ignore
        }
      };
      es.addEventListener("list_updated", handle);
      es.addEventListener("list_deleted", handle);
      es.addEventListener("ping", handle);
      es.onerror = () => {
        es?.close();
        es = null;
        if (!stopped) {
          retryTimer = window.setTimeout(open, 1000);
        }
      };
    };
    open();
    return () => {
      stopped = true;
      if (retryTimer) window.clearTimeout(retryTimer);
      es?.close();
    };
  }, [onEvent]);
}
