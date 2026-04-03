import { createSignal, onCleanup } from "solid-js";
import { getToken } from "./auth";

export function createRealtimeConnection() {
  const [data, setData] = createSignal<any>(null);
  const [connected, setConnected] = createSignal(false);
  let ws: WebSocket | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout>;
  let stopped = false;

  function getWsBase() {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${window.location.host}`;
  }

  function connect() {
    if (stopped) return;

    const token = getToken();
    if (!token) {
      // No token yet, retry later
      reconnectTimer = setTimeout(connect, 2000);
      return;
    }

    try {
      ws = new WebSocket(`${getWsBase()}/api/ws/live?token=${encodeURIComponent(token)}`);
    } catch {
      reconnectTimer = setTimeout(connect, 5000);
      return;
    }

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        setData(JSON.parse(event.data));
      } catch {}
    };

    ws.onclose = () => {
      setConnected(false);
      if (!stopped) {
        reconnectTimer = setTimeout(connect, 5000);
      }
    };

    ws.onerror = () => {
      ws?.close();
    };
  }

  connect();

  onCleanup(() => {
    stopped = true;
    clearTimeout(reconnectTimer);
    ws?.close();
  });

  return { data, connected };
}
