import { create } from 'zustand';
import api from '@/services/api';

interface AuthState {
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  login: (password: string) => Promise<void>;
  setup: (password: string) => Promise<void>;
  logout: () => Promise<void>;
  checkAuth: () => Promise<void>;
  clearError: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  isLoading: true,
  error: null,

  login: async (password: string) => {
    try {
      set({ isLoading: true, error: null });
      await api.login({ password });
      set({ isAuthenticated: true, isLoading: false });
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false });
      throw error;
    }
  },

  setup: async (password: string) => {
    try {
      set({ isLoading: true, error: null });
      await api.setup({ password });
      set({ isAuthenticated: true, isLoading: false });
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false });
      throw error;
    }
  },

  logout: async () => {
    try {
      await api.logout();
      set({ isAuthenticated: false });
    } catch (error) {
      console.error('Logout failed:', error);
      set({ isAuthenticated: false });
    }
  },

  checkAuth: async () => {
    try {
      set({ isLoading: true });
      const status = await api.checkInit();
      
      if (!status.initialized) {
        // Need to setup
        set({ isAuthenticated: false, isLoading: false });
        return;
      }

      if (api.getToken()) {
        const isValid = await api.verifyToken();
        set({ isAuthenticated: isValid, isLoading: false });
      } else {
        set({ isAuthenticated: false, isLoading: false });
      }
    } catch (error) {
      set({ isAuthenticated: false, isLoading: false });
    }
  },

  clearError: () => set({ error: null }),
}));
