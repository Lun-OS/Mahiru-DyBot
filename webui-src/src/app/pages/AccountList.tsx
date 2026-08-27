import { useEffect, useState, useMemo } from 'react';
import { useRouter } from '@/lib/router';
import {
  Search,
  Filter,
  Wifi,
  WifiOff,
  Loader2,
  ExternalLink,
  Plus,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { useAccountStore } from '@/stores/accountStore';
import api from '@/services/api';
import { Account } from '@/types';

type StatusFilter = 'all' | 'online' | 'offline';

export function AccountList() {
  const { navigate } = useRouter();
  const { accounts, fetchAccounts } = useAccountStore();
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [showAddModal, setShowAddModal] = useState(false);
  const [newName, setNewName] = useState('');

  useEffect(() => {
    fetchBots();
  }, []);

  const fetchBots = async () => {
    try {
      setLoading(true);
      await fetchAccounts();
    } catch {
      toast.error('获取账号列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    if (!newName.trim()) {
      toast.error('请输入账号名称');
      return;
    }
    try {
      await api.createAccount(newName.trim());
      toast.success('账号创建成功');
      setShowAddModal(false);
      setNewName('');
      await fetchBots();
    } catch (err) {
      toast.error((err as Error).message || '创建失败');
    }
  };

  const filteredAccounts = useMemo(() => {
    return accounts.filter((acc) => {
      const matchesStatus =
        statusFilter === 'all' ||
        (statusFilter === 'online' && acc.state === 'online') ||
        (statusFilter === 'offline' && acc.state !== 'online');
      const searchLower = search.toLowerCase();
      const matchesSearch =
        !search ||
        (acc.name || '').toLowerCase().includes(searchLower) ||
        (acc.uid || '').toLowerCase().includes(searchLower);
      return matchesStatus && matchesSearch;
    });
  }, [accounts, search, statusFilter]);

  const onlineCount = accounts.filter((a) => a.state === 'online').length;
  const offlineCount = accounts.length - onlineCount;

  if (loading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='w-8 h-8 animate-spin text-[#165DFF] dark:text-white/60' />
      </div>
    );
  }

  return (
    <>
      <div className='w-full max-w-[1000px] mx-auto space-y-4 px-4'>
        <div className='flex flex-col sm:flex-row items-start sm:items-center gap-3'>
          <div className='relative flex-1 w-full'>
            <Search className='absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-600 dark:text-gray-400' />
            <input
              type='text'
              placeholder='搜索昵称、UID...'
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className='w-full pl-10 pr-4 py-2.5 bg-white/50 dark:bg-white/[0.03] border border-white/60 dark:border-white/10 rounded-xl text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-600 focus:ring-1 focus:ring-blue-200 dark:focus:ring-white/20 backdrop-blur-sm dark:backdrop-blur-md outline-none transition-all'
            />
          </div>
          <div className='flex items-center gap-2'>
            <button
              onClick={() => { setNewName(''); setShowAddModal(true); }}
              className='flex items-center gap-1.5 px-4 py-2.5 bg-[#165DFF] text-white rounded-xl text-sm hover:bg-[#0047FF] transition-all shadow-lg shadow-[#165DFF]/20 dark:bg-white dark:text-black dark:hover:bg-gray-200 dark:shadow-black/20 font-medium'
            >
              <Plus className='w-4 h-4' />
              添加账号
            </button>
            <Filter className='w-4 h-4 text-gray-600 dark:text-gray-400' />
            {(
              [
                { key: 'all', label: '全部', count: accounts.length },
                { key: 'online', label: '在线', count: onlineCount },
                { key: 'offline', label: '离线', count: offlineCount },
              ] as const
            ).map((filter) => (
              <button
                key={filter.key}
                onClick={() => setStatusFilter(filter.key)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                  statusFilter === filter.key
                    ? 'bg-[#165DFF] text-white dark:bg-white dark:text-black'
                    : 'bg-gray-100 text-gray-700 dark:bg-white/[0.03] dark:text-gray-400 border border-white/60 dark:border-white/10 hover:bg-gray-100 dark:hover:bg-white/[0.06]'
                }`}
              >
                {filter.label} ({filter.count})
              </button>
            ))}
          </div>
        </div>

        {filteredAccounts.length === 0 ? (
          <div className='flex flex-col items-center justify-center py-20'>
            <WifiOff className='w-12 h-12 text-gray-300 dark:text-gray-600 mb-4' />
            <p className='text-gray-400 dark:text-gray-500 text-sm'>
              {search || statusFilter !== 'all' ? '没有匹配的账号' : '暂无账号'}
            </p>
          </div>
        ) : (
          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            {filteredAccounts.map((acc) => (
              <AccountCard
                key={acc.id}
                account={acc}
                onClick={() => navigate(`/accounts/${acc.id}`)}
              />
            ))}
          </div>
        )}
      </div>

      {showAddModal && (
        <div className='fixed inset-0 z-50 flex items-center justify-center'>
          <div className='absolute inset-0 bg-black/50 backdrop-blur-sm' onClick={() => setShowAddModal(false)} />
          <div className='relative bg-white dark:bg-[#1a1a1a] rounded-2xl shadow-2xl border border-white/60 dark:border-white/10 w-full max-w-lg mx-4 max-h-[85vh] overflow-y-auto'>
            <div className='flex items-center justify-between p-5 border-b border-gray-100 dark:border-white/[0.06]'>
              <h2 className='text-lg font-semibold text-gray-900 dark:text-white'>添加账号</h2>
              <button
                onClick={() => setShowAddModal(false)}
                className='p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-white/[0.06] text-gray-500 hover:text-gray-900 dark:hover:text-white transition-colors'
              >
                <X className='w-5 h-5' />
              </button>
            </div>
            <div className='p-5 space-y-4'>
              <div>
                <label className='text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5 block'>账号名称</label>
                <input
                  type='text'
                  placeholder='例如: 我的抖音号'
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
                  className='w-full px-4 py-2.5 bg-white/50 dark:bg-white/[0.03] border border-white/60 dark:border-white/10 rounded-xl text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-600 focus:ring-1 focus:ring-blue-200 dark:focus:ring-white/20 backdrop-blur-sm outline-none transition-all'
                  autoFocus
                />
              </div>
            </div>
            <div className='flex justify-end gap-3 p-5 border-t border-gray-100 dark:border-white/[0.06]'>
              <button
                onClick={() => setShowAddModal(false)}
                className='px-5 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/[0.06] rounded-xl transition-colors'
              >
                取消
              </button>
              <button
                onClick={handleCreate}
                disabled={!newName.trim()}
                className='px-5 py-2 bg-[#165DFF] text-white rounded-xl text-sm hover:bg-[#0047FF] transition-all shadow-lg shadow-[#165DFF]/20 disabled:opacity-50 font-medium'
              >
                创建
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function AccountCard({ account, onClick }: { account: Account; onClick: () => void }) {
  const isOnline = account.state === 'online';
  const nickname = account.name || account.uid || '未命名';

  return (
    <div
      onClick={onClick}
      className='backdrop-blur-sm bg-white/60 dark:bg-black/40 dark:backdrop-blur-xl border border-white/60 dark:border-white/10 rounded-2xl p-4 cursor-pointer hover:bg-gray-100 dark:hover:bg-white/[0.04] hover:border-blue-200 dark:hover:border-white/[0.12] transition-all duration-300 group'
    >
      <div className='flex items-center gap-4'>
        <div className='relative shrink-0'>
          <div className='w-12 h-12 rounded-full bg-gradient-to-br from-[#165DFF] to-purple-600 flex items-center justify-center text-white font-bold text-lg border-2 border-white/60 dark:border-white/10 shadow-sm'>
            {nickname[0]?.toUpperCase() || 'D'}
          </div>
          <span
            className={`absolute bottom-0 right-0 w-3.5 h-3.5 rounded-full border-2 border-black ${
              isOnline ? 'bg-green-500 dark:bg-green' : 'bg-gray-400 dark:bg-gray-600'
            }`}
          />
        </div>

        <div className='flex-1 min-w-0'>
          <div className='flex items-center gap-2'>
            <h3 className='text-sm font-semibold text-gray-900 dark:text-white truncate'>
              {nickname}
            </h3>
            {isOnline ? (
              <span className='inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-blue-50 text-blue-700 dark:bg-white/[0.06] dark:text-gray-300'>
                <Wifi className='w-2.5 h-2.5' />
                在线
              </span>
            ) : (
              <span className='inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-blue-50 text-blue-700 dark:bg-white/[0.06] dark:text-gray-300'>
                <WifiOff className='w-2.5 h-2.5' />
                离线
              </span>
            )}
          </div>

          <div className='flex items-center gap-3 mt-1.5 text-xs text-gray-600 dark:text-gray-400'>
            {account.uid && (
              <span className='flex items-center gap-1'>
                <span className='text-[10px] px-1.5 py-0.5 rounded bg-blue-50 text-blue-700 dark:bg-white/[0.06] dark:text-gray-300 font-medium'>
                  UID
                </span>
                <span className='truncate max-w-[120px] font-mono'>{account.uid}</span>
              </span>
            )}
          </div>

          <p className='text-[11px] text-gray-500 mt-1 font-mono'>
            ID: {account.id}
          </p>
        </div>

        <ExternalLink className='w-4 h-4 text-gray-500 group-hover:text-[#165DFF] dark:group-hover:text-white transition-colors shrink-0' />
      </div>
    </div>
  );
}
