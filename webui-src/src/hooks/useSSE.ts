import { useEffect, useRef, useCallback } from 'react';
import api from '@/services/api';
import { SSEEvent } from '@/types';

interface UseSSEOptions {
  onEvent?: (event: SSEEvent) => void;
  onAccountStatus?: (accountId: string, status: string) => void;
  onSDKReady?: (accountId: string) => void;
  enabled?: boolean;
}

export function useSSE(options: UseSSEOptions = {}) {
  const { onEvent, onAccountStatus, onSDKReady, enabled = true } = options;
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (!enabled || eventSourceRef.current) {
      return;
    }

    const token = api.getToken();
    if (!token) {
      return;
    }

    const url = `/api/webui/events?token=${encodeURIComponent(token)}`;
    const eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
      try {
        const data: SSEEvent = JSON.parse(event.data);
        
        if (onEvent) {
          onEvent(data);
        }

        switch (data.type) {
          case 'account_status':
            if (onAccountStatus && typeof data.data.account_id === 'string' && typeof data.data.status === 'string') {
              onAccountStatus(data.data.account_id, data.data.status);
            }
            break;
          case 'sdk_ready':
            if (onSDKReady && typeof data.data.account_id === 'string') {
              onSDKReady(data.data.account_id);
            }
            break;
        }
      } catch (error) {
        console.error('Failed to parse SSE event:', error);
      }
    };

    eventSource.onerror = () => {
      console.log('SSE connection error, will reconnect...');
      eventSource.close();
      eventSourceRef.current = null;

      // Reconnect after 3 seconds
      reconnectTimeoutRef.current = setTimeout(() => {
        connect();
      }, 3000);
    };

    eventSourceRef.current = eventSource;
  }, [enabled, onEvent, onAccountStatus, onSDKReady]);

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

    // Listen for auth expiration
    const handleAuthExpired = () => {
      disconnect();
    };
    window.addEventListener('auth-expired', handleAuthExpired);

    return () => {
      disconnect();
      window.removeEventListener('auth-expired', handleAuthExpired);
    };
  }, [connect, disconnect]);

  return { connect, disconnect };
}
