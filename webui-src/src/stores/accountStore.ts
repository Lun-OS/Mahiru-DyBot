import { create } from 'zustand';
import api from '@/services/api';
import { Account } from '@/types';

type AccountStateType = Account['state'];

interface AccountStoreState {
  accounts: Account[];
  selectedId: string | null;
  isLoading: boolean;
  error: string | null;
  fetchAccounts: () => Promise<void>;
  selectAccount: (id: string | null) => void;
  createAccount: (name: string) => Promise<Account>;
  deleteAccount: (id: string) => Promise<void>;
  renameAccount: (id: string, name: string) => Promise<void>;
  startAccount: (id: string) => Promise<void>;
  stopAccount: (id: string) => Promise<void>;
  updateAccountStatus: (id: string, state: string) => void;
  clearError: () => void;
}

export const useAccountStore = create<AccountStoreState>((set) => ({
  accounts: [],
  selectedId: null,
  isLoading: false,
  error: null,

  fetchAccounts: async () => {
    try {
      set({ isLoading: true, error: null });
      const accounts = await api.getAccounts();
      set({ accounts, isLoading: false });
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false });
    }
  },

  selectAccount: (id) => {
    set({ selectedId: id });
  },

  createAccount: async (name: string) => {
    try {
      set({ error: null });
      const account = await api.createAccount(name);
      set((state) => ({ accounts: [...state.accounts, account] }));
      return account;
    } catch (error) {
      set({ error: (error as Error).message });
      throw error;
    }
  },

  deleteAccount: async (id: string) => {
    try {
      set({ error: null });
      await api.deleteAccount(id);
      set((state) => ({
        accounts: state.accounts.filter((a) => a.id !== id),
        selectedId: state.selectedId === id ? null : state.selectedId,
      }));
    } catch (error) {
      set({ error: (error as Error).message });
      throw error;
    }
  },

  renameAccount: async (id: string, name: string) => {
    try {
      set({ error: null });
      await api.renameAccount(id, name);
      set((state) => ({
        accounts: state.accounts.map((a) =>
          a.id === id ? { ...a, name } : a
        ),
      }));
    } catch (error) {
      set({ error: (error as Error).message });
      throw error;
    }
  },

  startAccount: async (id: string) => {
    try {
      set({ error: null });
      await api.startAccount(id);
      set((state) => ({
        accounts: state.accounts.map((a) =>
          a.id === id ? { ...a, state: 'starting' as AccountStateType } : a
        ),
      }));
    } catch (error) {
      set({ error: (error as Error).message });
      throw error;
    }
  },

  stopAccount: async (id: string) => {
    try {
      set({ error: null });
      await api.stopAccount(id);
      set((state) => ({
        accounts: state.accounts.map((a) =>
          a.id === id ? { ...a, state: 'stopped' as AccountStateType } : a
        ),
      }));
    } catch (error) {
      set({ error: (error as Error).message });
      throw error;
    }
  },

  updateAccountStatus: (id: string, state: string) => {
    set((s) => ({
      accounts: s.accounts.map((a) =>
        a.id === id ? { ...a, state: state as AccountStateType } : a
      ),
    }));
  },

  clearError: () => set({ error: null }),
}));
