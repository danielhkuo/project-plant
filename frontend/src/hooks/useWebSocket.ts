"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { TelemetryEvent, Alert, WebSocketMessage } from "@/lib/types";

const WS_URL =
  process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8081";
const MAX_RECONNECT_DELAY = 30_000;
const DEMO_MODE = process.env.NEXT_PUBLIC_DEMO_MODE === "true";

interface UseWebSocketOptions {
  deviceIds?: string[];
  onReading?: (event: TelemetryEvent) => void;
  onAlert?: (alert: Alert) => void;
}

export function useWebSocket({
  deviceIds,
  onReading,
  onAlert,
}: UseWebSocketOptions = {}) {
  const [connected, setConnected] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectDelay = useRef(1000);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const onReadingRef = useRef(onReading);
  const onAlertRef = useRef(onAlert);
  onReadingRef.current = onReading;
  onAlertRef.current = onAlert;

  const connect = useCallback(() => {
    const params = deviceIds?.length
      ? `?devices=${deviceIds.join(",")}`
      : "";
    const url = `${WS_URL}/api/v1/ws/events${params}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      setReconnecting(false);
      reconnectDelay.current = 1000;
    };

    ws.onmessage = (event) => {
      try {
        const msg: WebSocketMessage = JSON.parse(event.data);
        if (msg.type === "reading" && onReadingRef.current) {
          onReadingRef.current(msg.payload as TelemetryEvent);
        } else if (msg.type === "alert" && onAlertRef.current) {
          onAlertRef.current(msg.payload as Alert);
        }
      } catch {
        // Ignore malformed messages
      }
    };

    ws.onclose = () => {
      setConnected(false);
      setReconnecting(true);
      reconnectTimer.current = setTimeout(() => {
        reconnectDelay.current = Math.min(
          reconnectDelay.current * 2,
          MAX_RECONNECT_DELAY
        );
        connect();
      }, reconnectDelay.current);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [deviceIds]);

  useEffect(() => {
    // In demo mode there is no backend — skip connection and report
    // a permanently "connected" state so the UI badge reads [LIVE].
    // The MockProvider drives live updates via a polling tick instead.
    if (DEMO_MODE) {
      setConnected(true);
      setReconnecting(false);
      return;
    }
    connect();
    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { connected, reconnecting };
}
