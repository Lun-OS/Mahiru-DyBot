import { useEffect, useRef, useCallback } from 'react';
import api from '@/services/api';

interface SSEMessage {
  type: string;
  data: Record<string, unknown>;
}

interface UseSSEOptions {
  onAccountStatus?: (accountId: string, status: string) => void;
  onLog?: (accountId: string, message: string) => void;
  enabled?: boolean;
}

export function useSSE(options: UseSSEOptions = {}) {
  const { onAccountStatus, onLog, enabled = true } = options;
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const callbacksRef = useRef({ onAccountStatus, onLog });
  callbacksRef.current = { onAccountStatus, onLog };

  const connect = useCallback(() => {
    if (!enabled || eventSourceRef.current) return;

    const token = api.getToken();
    if (!token) return;

    const url = `/api/webui/events?token=${encodeURIComponent(token)}`;
    const eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
      try {
        const msg: SSEMessage = JSON.parse(event.data);
        const cbs = callbacksRef.current;

        switch (msg.type) {
          case 'account_status':
            if (cbs.onAccountStatus && typeof msg.data.account_id === 'string' && typeof msg.data.status === 'string') {
              cbs.onAccountStatus(msg.data.account_id, msg.data.status);
            }
            break;
          case 'log':
            if (cbs.onLog && typeof msg.data.account_id === 'string' && typeof msg.data.message === 'string') {
              cbs.onLog(msg.data.account_id, msg.data.message);
            }
            break;
        }
      } catch {}
    };

    eventSource.onerror = () => {
      eventSource.close();
      eventSourceRef.current = null;
      reconnectTimeoutRef.current = setTimeout(connect, 3000);
    };

    eventSourceRef.current = eventSource;
  }, [enabled]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  useEffect(() => {
    connect();
    const handleAuthExpired = () => disconnect();
    window.addEventListener('auth-expired', handleAuthExpired);
    return () => {
      disconnect();
      window.removeEventListener('auth-expired', handleAuthExpired);
    };
  }, [connect, disconnect]);

  return { connect, disconnect };
}
