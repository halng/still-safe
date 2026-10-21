import { createContext, ReactNode, useContext, useEffect, useMemo, useRef } from "react";
import { socketUrl } from "./api";
import type { LiveEvent } from "./types";

type Listener = (event: LiveEvent) => void;

const LiveContext = createContext<((listener: Listener) => () => void) | null>(null);

export function LiveProvider({ token, children }: { token: string; children: ReactNode }) {
  const listeners = useRef(new Set<Listener>());
  useEffect(() => {
    const socket = new WebSocket(socketUrl(token));
    socket.onmessage = (message) => {
      try {
        const event = JSON.parse(String(message.data)) as LiveEvent;
        listeners.current.forEach((listener) => listener(event));
      } catch {
        // Ignore a malformed frame.
      }
    };
    return () => socket.close();
  }, [token]);

  const subscribe = useMemo(
    () => (listener: Listener) => {
      listeners.current.add(listener);
      return () => listeners.current.delete(listener);
    },
    [],
  );
  return <LiveContext.Provider value={subscribe}>{children}</LiveContext.Provider>;
}

export function useLive(listener: Listener) {
  const subscribe = useContext(LiveContext);
  useEffect(() => {
    if (!subscribe) return;
    return subscribe(listener);
  }, [subscribe, listener]);
}
