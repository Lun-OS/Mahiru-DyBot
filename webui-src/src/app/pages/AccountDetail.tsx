import { useEffect, useState, useRef, useCallback } from 'react';
import { useRouter } from '@/lib/router';
import { useParams } from '@/App';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useAccountStore } from '@/stores/accountStore';
import api from '@/services/api';
import { AccountInfo, Adapter, AdapterType } from '@/types';
import { toast } from 'sonner';
import { motion, AnimatePresence } from 'motion/react';
import {
  ArrowLeft,
  Play,
  StopCircle,
  Bug,
  Settings,
  RefreshCw,
  Loader2,
  Copy,
  Check,
  Terminal,
  Monitor,
  Plus,
  Trash2,
  Wifi,
  ArrowRight,
  Globe,
  X,
  Save,
  Activity,
  Send,
  Search,
  Download,
  Network,
  FileText,
} from 'lucide-react';

const ADAPTER_LABELS: Record<AdapterType, { label: string; icon: typeof Wifi; desc: string }> = {
  forward_ws: { label: '正向 WebSocket', icon: Wifi, desc: '作为客户端连接' },
  reverse_ws: { label: '反向 WebSocket', icon: ArrowRight, desc: '作为服务端被连接' },
  http_server: { label: 'HTTP 服务', icon: Globe, desc: '作为 http 接口' },
};

const ADAPTER_ICONS: Record<AdapterType, React.ReactNode> = {
  forward_ws: <Send className='w-5 h-5 text-blue-500' />,
  reverse_ws: <Activity className='w-5 h-5 text-purple-500' />,
  http_server: <Network className='w-5 h-5 text-green-500' />,
};

const inputClass =
  'w-full px-3 py-2 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-[#165DFF]/50 focus:border-transparent';

export function AccountDetail() {
  const { navigate } = useRouter();
  const { id } = useParams();
  const { accounts, startAccount, stopAccount } = useAccountStore();
  const [accountInfo, setAccountInfo] = useState<AccountInfo | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('overview');

  const account = accounts.find((a) => a.id === id);

  useEffect(() => {
    if (!id) return;
    loadAccountInfo();
  }, [id]);

  const loadAccountInfo = async () => {
    if (!id) return;
    setIsLoading(true);
    try {
      const info = await api.getAccountInfo(id);
      setAccountInfo(info);
    } catch {
      console.error('Failed to load account info');
    } finally {
      setIsLoading(false);
    }
  };

  if (!id) {
    navigate('/accounts');
    return null;
  }

  return (
    <div className='space-y-6 px-4 max-w-[1200px] mx-auto'>
      <div className='flex items-center justify-between'>
        <div className='flex items-center space-x-4'>
          <button
            onClick={() => navigate('/accounts')}
            className='p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-white/[0.06] text-gray-500 dark:text-gray-400 transition-colors'
          >
            <ArrowLeft className='w-5 h-5' />
          </button>
          <div>
            <h1 className='text-2xl font-bold text-gray-900 dark:text-white'>{account?.name || '加载中...'}</h1>
            <p className='text-gray-500 dark:text-gray-400'>UID: {account?.uid || '--'}</p>
          </div>
        </div>
        <div className='flex items-center gap-2'>
          {account?.state === 'online' ? (
            <motion.button
              onClick={() => id && stopAccount(id)}
              className='flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-600 bg-red-50 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/30 rounded-lg transition-colors'
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
            >
              <StopCircle className='w-4 h-4' />
              停止
            </motion.button>
          ) : (
            <motion.button
              onClick={() => id && startAccount(id)}
              disabled={account?.state === 'starting'}
              className='flex items-center gap-2 px-4 py-2 text-sm font-medium bg-[#165DFF] text-white hover:bg-[#0047FF] dark:bg-white dark:text-black dark:hover:bg-gray-200 rounded-lg transition-colors disabled:opacity-50'
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
            >
              {account?.state === 'starting' ? (
                <Loader2 className='w-4 h-4 animate-spin' />
              ) : (
                <Play className='w-4 h-4' />
              )}
              启动
            </motion.button>
          )}
          <motion.button
            onClick={loadAccountInfo}
            className='p-2 text-gray-500 dark:text-gray-400 hover:text-[#165DFF] dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/[0.05] rounded-lg transition-colors'
            whileHover={{ rotate: 180 }}
            transition={{ duration: 0.3 }}
          >
            <RefreshCw className='w-4 h-4' />
          </motion.button>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className='bg-white/40 dark:bg-black/20 backdrop-blur-sm rounded-2xl p-1.5 border border-white/40 dark:border-white/10 mb-4 md:mb-8 w-full md:w-fit mx-auto overflow-x-auto'>
          {[
            { value: 'overview', label: '概览', icon: Monitor },
            { value: 'adapters', label: '连接管理', icon: Network },
            { value: 'logs', label: '日志', icon: FileText },
            { value: 'remote', label: '远程控制', icon: Monitor },
            { value: 'debug', label: '调试', icon: Bug },
          ].map((tab) => (
            <TabsTrigger
              key={tab.value}
              value={tab.value}
              className='h-9 px-4 md:px-6 data-[state=active]:bg-[#165DFF]/10 data-[state=active]:backdrop-blur-md data-[state=active]:shadow-sm data-[state=active]:rounded-xl data-[state=active]:text-[#165DFF] dark:data-[state=active]:bg-white/10 dark:data-[state=active]:text-white text-gray-600 dark:text-gray-400 hover:text-[#165DFF] dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/[0.04] font-medium transition-all'
            >
              <tab.icon className='w-4 h-4 mr-2' />
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value='overview' className='mt-0'>
          <OverviewTab accountInfo={accountInfo} isLoading={isLoading} />
        </TabsContent>
        <TabsContent value='adapters' className='mt-0'>
          <AdaptersTab accountId={id} />
        </TabsContent>
        <TabsContent value='logs' className='mt-0'>
          <LogsTab accountId={id} />
        </TabsContent>
        <TabsContent value='remote' className='mt-0'>
          <RemoteTab accountId={id} />
        </TabsContent>
        <TabsContent value='debug' className='mt-0'>
          <DebugTab accountId={id} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

// --- Overview ---
function OverviewTab({ accountInfo, isLoading }: { accountInfo: AccountInfo | null; isLoading: boolean }) {
  const { id } = useParams();
  const [editName, setEditName] = useState('');
  const [editVPW, setEditVPW] = useState('');
  const [editVPH, setEditVPH] = useState('');
  const [editUA, setEditUA] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (accountInfo) {
      setEditName(accountInfo.name || '');
      setEditVPW(String(accountInfo.viewport_width || 1920));
      setEditVPH(String(accountInfo.viewport_height || 1080));
      setEditUA(accountInfo.custom_ua || '');
    }
  }, [accountInfo]);

  const handleSave = async () => {
    if (!id) return;
    setSaving(true);
    try {
      const res = await api.updateAccountSettings(id, {
        name: editName.trim() || undefined,
        viewport_width: parseInt(editVPW) || undefined,
        viewport_height: parseInt(editVPH) || undefined,
        custom_ua: editUA,
      });
      if (res.ok) {
        toast.success('设置已保存');
      } else {
        toast.error(res.error || '保存失败');
      }
    } catch {
      toast.error('保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <RefreshCw className='w-8 h-8 animate-spin text-[#165DFF] dark:text-white/60' />
      </div>
    );
  }
  if (!accountInfo) {
    return (
      <div className='flex items-center justify-center h-64 text-gray-500 dark:text-gray-400 text-sm'>
        无法获取账号信息
      </div>
    );
  }

  const statCards = [
    { label: '状态', value: accountInfo.state === 'online' ? '在线' : '离线', icon: Activity },
    { label: 'SDK 就绪', value: accountInfo.sdk_ready ? '是' : '否', icon: Check },
    { label: '视口', value: `${accountInfo.viewport?.width || 0}x${accountInfo.viewport?.height || 0}`, icon: Monitor },
  ];

  return (
    <motion.div className='space-y-6' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.5 }}>
      <div className='grid grid-cols-2 md:grid-cols-3 gap-3'>
        {statCards.map((stat, idx) => (
          <motion.div
            key={idx}
            className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl rounded-xl border border-gray-100 dark:border-gray-800 overflow-hidden hover:shadow-md transition-shadow'
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: idx * 0.08 }}
          >
            <div className='h-1 bg-gradient-to-r from-[#165DFF]/20 to-[#165DFF]/5 dark:from-white/10 dark:to-white/5' />
            <div className='p-4'>
              <div className='flex items-center justify-between mb-3'>
                <span className='text-xs text-gray-500 dark:text-gray-400 font-medium'>{stat.label}</span>
                <div className='w-8 h-8 rounded-lg bg-white dark:bg-white/[0.03] flex items-center justify-center'>
                  <stat.icon className='w-4 h-4 text-gray-700 dark:text-gray-300' />
                </div>
              </div>
              <p className='text-2xl font-bold text-gray-900 dark:text-white'>{stat.value}</p>
            </div>
          </motion.div>
        ))}
      </div>

      <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
        <motion.div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl rounded-xl border border-gray-100 dark:border-gray-800 p-5' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.3 }}>
          <h3 className='text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2'>
            <Settings className='w-4 h-4 text-gray-900 dark:text-white' />
            系统信息
          </h3>
          <div className='space-y-3'>
            {[
              { label: '昵称', value: accountInfo.nickname || '--' },
              { label: 'UID', value: accountInfo.uid || '--' },
              { label: '模块 ID', value: accountInfo.mod_id || '--' },
              { label: '状态', value: accountInfo.state === 'online' ? '在线' : '离线' },
            ].map((item, i) => (
              <div key={i} className='flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-800 last:border-0'>
                <span className='text-sm text-gray-500 dark:text-gray-400'>{item.label}</span>
                <span className='text-sm font-medium text-gray-900 dark:text-white'>{item.value}</span>
              </div>
            ))}
          </div>
        </motion.div>

        <motion.div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl rounded-xl border border-gray-100 dark:border-gray-800 p-5' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.4 }}>
          <h3 className='text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2'>
            <Activity className='w-4 h-4 text-gray-900 dark:text-white' />
            SDK 信息
          </h3>
          <div className='space-y-3'>
            {[
              { label: 'SDK 就绪', value: accountInfo.sdk_ready ? '是' : '否' },
              { label: '模块 ID', value: accountInfo.mod_id || '--' },
              { label: '实际视口', value: accountInfo.actual_viewport_width ? `${accountInfo.actual_viewport_width}x${accountInfo.actual_viewport_height}` : '--' },
              { label: '实际 UA', value: accountInfo.actual_ua || '(未启动)' },
            ].map((item, i) => (
              <div key={i} className='flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-800 last:border-0'>
                <span className='text-sm text-gray-500 dark:text-gray-400'>{item.label}</span>
                <span className='text-sm font-medium text-gray-900 dark:text-white truncate max-w-[200px]' title={String(item.value)}>{item.value}</span>
              </div>
            ))}
          </div>
        </motion.div>
      </div>

      {/* Editable Settings */}
      <motion.div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl rounded-xl border border-gray-100 dark:border-gray-800 p-5' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.5 }}>
        <h3 className='text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2'>
          <Settings className='w-4 h-4 text-gray-900 dark:text-white' />
          容器设置
          <span className='text-xs text-gray-500 dark:text-gray-400 font-normal ml-2'>
            {accountInfo.actual_ua ? '修改后需重启账号生效' : '启动时使用这些设置'}
          </span>
        </h3>
        <div className='space-y-4'>
          <div>
            <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>账号名称</label>
            <input
              type='text'
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
              placeholder='账号名称'
              className='w-full px-3 py-2 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-[#165DFF]/50 focus:border-transparent'
            />
          </div>
          <div className='grid grid-cols-2 gap-4'>
            <div>
              <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>视口宽度 (px)</label>
              <input
                type='number'
                value={editVPW}
                onChange={(e) => setEditVPW(e.target.value)}
                placeholder='1920'
                min='320'
                max='7680'
                className='w-full px-3 py-2 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-[#165DFF]/50 focus:border-transparent'
              />
            </div>
            <div>
              <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>视口高度 (px)</label>
              <input
                type='number'
                value={editVPH}
                onChange={(e) => setEditVPH(e.target.value)}
                placeholder='1080'
                min='240'
                max='4320'
                className='w-full px-3 py-2 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-[#165DFF]/50 focus:border-transparent'
              />
            </div>
          </div>
          <div>
            <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>自定义 User-Agent</label>
            <textarea
              value={editUA}
              onChange={(e) => setEditUA(e.target.value)}
              placeholder='留空使用默认 UA'
              rows={3}
              className='w-full px-3 py-2 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-[#165DFF]/50 focus:border-transparent resize-none font-mono'
            />
            <p className='text-xs text-gray-500 dark:text-gray-400 mt-1'>
              {editUA ? `已设置自定义 UA (${editUA.length} 字符)` : '留空将使用系统默认 User-Agent'}
            </p>
          </div>
          <div className='flex justify-end'>
            <motion.button
              onClick={handleSave}
              disabled={saving}
              className='flex items-center gap-2 px-4 py-2 bg-[#165DFF] text-white rounded-lg text-sm hover:bg-[#0047FF] transition-colors disabled:opacity-50'
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
            >
              {saving ? <Loader2 className='w-4 h-4 animate-spin' /> : <Save className='w-4 h-4' />}
              保存设置
            </motion.button>
          </div>
        </div>
      </motion.div>

      <motion.div className='flex justify-end' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.6 }}>
        <motion.button
          onClick={() => window.location.reload()}
          className='flex items-center gap-2 px-4 py-2 text-sm text-gray-500 dark:text-gray-400 hover:text-[#165DFF] dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors'
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
        >
          <RefreshCw className='w-4 h-4' />
          刷新数据
        </motion.button>
      </motion.div>
    </motion.div>
  );
}

// --- Adapters (ProxyTab style) ---
function AdaptersTab({ accountId }: { accountId: string }) {
  const [adapters, setAdapters] = useState<Adapter[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [editing, setEditing] = useState<Adapter | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const [formType, setFormType] = useState<AdapterType>('reverse_ws');
  const [formData, setFormData] = useState({ name: '', url: '', token: '', enabled: true });

  const load = async () => {
    try {
      const data = await api.getAccountAdapters(accountId);
      setAdapters(data);
    } catch {}
    setLoading(false);
  };

  useEffect(() => { load(); }, [accountId]);

  const resetForm = () => {
    setFormData({ name: '', url: '', token: '', enabled: true });
    setFormType('reverse_ws');
    setEditing(null);
  };

  const handleAdd = async () => {
    if (!formData.name.trim()) { toast.error('请输入适配器名称'); return; }
    try {
      setActionLoading('add');
      await api.createAccountAdapter(accountId, { type: formType, ...formData });
      toast.success('适配器添加成功');
      setShowAddDialog(false);
      resetForm();
      load();
    } catch (err) { toast.error((err as Error).message || '添加失败'); }
    finally { setActionLoading(null); }
  };

  const handleUpdate = async () => {
    if (!editing) return;
    try {
      setActionLoading('update');
      await api.updateAccountAdapter(accountId, editing.id, formData);
      toast.success('适配器更新成功');
      setShowEditDialog(false);
      resetForm();
      load();
    } catch (err) { toast.error((err as Error).message || '更新失败'); }
    finally { setActionLoading(null); }
  };

  const handleDelete = async (aid: string) => {
    if (!confirm(`确定要删除适配器吗？`)) return;
    try {
      setActionLoading(`delete-${aid}`);
      await api.deleteAccountAdapter(accountId, aid);
      toast.success('适配器已删除');
      load();
    } catch (err) { toast.error((err as Error).message || '删除失败'); }
    finally { setActionLoading(null); }
  };

  const handleToggle = async (adapter: Adapter) => {
    try {
      setActionLoading(`toggle-${adapter.id}`);
      await api.updateAccountAdapter(accountId, adapter.id, { enabled: !adapter.enabled });
      toast.success(adapter.enabled ? '已禁用' : '已启用');
      load();
    } catch (err) { toast.error((err as Error).message || '操作失败'); }
    finally { setActionLoading(null); }
  };

  const openEditDialog = (adapter: Adapter) => {
    setEditing(adapter);
    setFormType(adapter.type);
    setFormData({ name: adapter.name, url: adapter.url || '', token: adapter.token || '', enabled: adapter.enabled });
    setShowEditDialog(true);
  };

  const getConfigSummary = (adapter: Adapter): string => {
    if (adapter.type === 'forward_ws') {
      return `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/event`;
    }
    return adapter.url || '-';
  };

  if (loading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <RefreshCw className='w-8 h-8 animate-spin text-[#165DFF] dark:text-white/60' />
      </div>
    );
  }

  return (
    <motion.div className='space-y-6' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-3'>
          <Network className='w-6 h-6 text-[#165DFF] dark:text-white' />
          <h2 className='text-lg font-bold text-gray-900 dark:text-white'>接口代理</h2>
          <span className='px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-400'>
            {adapters.length} 个适配器
          </span>
        </div>
        <motion.button
          onClick={() => { resetForm(); setShowAddDialog(true); }}
          className='flex items-center gap-2 px-4 py-2 bg-[#165DFF] hover:bg-[#4080ff] text-white rounded-lg font-medium transition-colors'
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
        >
          <Plus className='w-4 h-4' />
          添加适配器
        </motion.button>
      </div>

      {adapters.length === 0 ? (
        <motion.div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl p-12 rounded-xl border border-gray-100 dark:border-gray-800 text-center' initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
          <Network className='w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-3' />
          <p className='text-gray-500 dark:text-gray-400'>暂无适配器</p>
          <p className='text-xs text-gray-400 dark:text-gray-500 mt-1'>点击上方按钮添加新的适配器</p>
        </motion.div>
      ) : (
        <div className='grid grid-cols-1 lg:grid-cols-2 gap-4'>
          <AnimatePresence>
            {adapters.map((adapter, index) => (
              <motion.div
                key={adapter.id}
                className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl p-5 rounded-xl border border-gray-100 dark:border-gray-800 hover:border-[#165DFF]/30 dark:hover:border-white/20 transition-colors'
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.95 }}
                transition={{ delay: index * 0.05 }}
              >
                <div className='flex items-start justify-between mb-3'>
                  <div className='flex items-center gap-3'>
                    <div className='p-2 rounded-lg bg-gray-50 dark:bg-white/5'>
                      {ADAPTER_ICONS[adapter.type] || <Network className='w-5 h-5 text-gray-500' />}
                    </div>
                    <div>
                      <h3 className='font-semibold text-gray-900 dark:text-white'>{adapter.name}</h3>
                      <p className='text-xs text-gray-500 dark:text-gray-400'>{ADAPTER_LABELS[adapter.type]?.label || adapter.type}</p>
                    </div>
                  </div>
                  <span className={`px-2 py-1 rounded-full text-xs font-medium flex-shrink-0 ${
                    adapter.enabled
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-400'
                  }`}>
                    {adapter.enabled ? '已启用' : '未启用'}
                  </span>
                </div>

                <div className='mb-3 px-3 py-2 bg-gray-50 dark:bg-white/5 rounded-lg'>
                  <code className='text-xs text-gray-600 dark:text-gray-400 break-all'>{getConfigSummary(adapter)}</code>
                </div>

                <div className='flex items-center gap-2 pt-3 border-t border-gray-100 dark:border-gray-800'>
                  <motion.button
                    onClick={() => handleToggle(adapter)}
                    disabled={actionLoading === `toggle-${adapter.id}`}
                    className={`flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                      adapter.enabled
                        ? 'bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400'
                        : 'bg-green-50 text-green-600 hover:bg-green-100 dark:bg-green-900/20 dark:text-green-400'
                    }`}
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                  >
                    {actionLoading === `toggle-${adapter.id}` ? <RefreshCw className='w-3 h-3 animate-spin' /> : adapter.enabled ? <StopCircle className='w-3 h-3' /> : <Play className='w-3 h-3' />}
                    {adapter.enabled ? '禁用' : '启用'}
                  </motion.button>
                  <motion.button
                    onClick={() => openEditDialog(adapter)}
                    className='flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-white/10 dark:text-gray-400 dark:hover:bg-white/20 transition-colors'
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                  >
                    <Settings className='w-3 h-3' />
                    编辑
                  </motion.button>
                  <motion.button
                    onClick={() => handleDelete(adapter.id)}
                    disabled={actionLoading === `delete-${adapter.id}`}
                    className='flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 transition-colors ml-auto'
                    whileHover={{ scale: 1.02 }}
                    whileTap={{ scale: 0.98 }}
                  >
                    {actionLoading === `delete-${adapter.id}` ? <RefreshCw className='w-3 h-3 animate-spin' /> : <Trash2 className='w-3 h-3' />}
                    删除
                  </motion.button>
                </div>
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
      )}

      {/* Add/Edit Dialog */}
      <AnimatePresence>
        {(showAddDialog || showEditDialog) && (
          <motion.div
            className='fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm'
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={(e) => { if (e.target === e.currentTarget) { setShowAddDialog(false); setShowEditDialog(false); resetForm(); } }}
          >
            <motion.div
              className='bg-white dark:bg-[#1D2129] rounded-2xl shadow-2xl w-full max-w-lg max-h-[90vh] overflow-y-auto'
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
            >
              <div className='flex items-center justify-between p-5 border-b border-gray-100 dark:border-gray-800'>
                <h3 className='text-lg font-bold text-gray-900 dark:text-white'>
                  {showEditDialog ? '编辑适配器' : '添加适配器'}
                </h3>
                <button onClick={() => { setShowAddDialog(false); setShowEditDialog(false); resetForm(); }} className='p-1.5 hover:bg-gray-100 dark:hover:bg-white/10 rounded-lg transition-colors'>
                  <X className='w-5 h-5 text-gray-500' />
                </button>
              </div>
              <div className='p-5 space-y-4'>
                {!showEditDialog && (
                  <div>
                    <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3'>选择适配器类型</label>
                    <div className='grid grid-cols-2 gap-3'>
                      {Object.entries(ADAPTER_LABELS).map(([key, info]) => (
                        <button
                          key={key}
                          type='button'
                          onClick={() => setFormType(key as AdapterType)}
                          className={`flex items-center gap-3 p-4 rounded-xl border-2 transition-all ${
                            formType === key
                              ? 'border-[#165DFF] bg-blue-50 dark:bg-blue-900/20 shadow-md'
                              : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 hover:bg-gray-50 dark:hover:bg-white/5'
                          }`}
                        >
                          {ADAPTER_ICONS[key as AdapterType]}
                          <div className='flex flex-col items-start'>
                            <span className={`text-sm font-semibold ${formType === key ? 'text-[#165DFF]' : 'text-gray-700 dark:text-gray-300'}`}>
                              {info.label}
                            </span>
                            <span className='text-[10px] text-gray-500 dark:text-gray-400 leading-tight mt-0.5'>
                              {info.desc}
                            </span>
                          </div>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
                <div>
                  <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>名称 *</label>
                  <input
                    type='text'
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder='例如：my-ws-client'
                    className={inputClass}
                    disabled={showEditDialog}
                  />
                </div>
                {(formType === 'reverse_ws' || formType === 'http_server') && (
                  <div>
                    <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>
                      {formType === 'reverse_ws' ? '连接地址 (URL)' : '回调 URL'}
                    </label>
                    <input
                      type='text'
                      value={formData.url}
                      onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                      placeholder={formType === 'reverse_ws' ? 'wss://your-server.com:123/path' : 'http://your-server.com:8080/webhook'}
                      className={`${inputClass} font-mono`}
                    />
                  </div>
                )}
                <div>
                  <label className='block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2'>Token（可选）</label>
                  <input
                    type='text'
                    value={formData.token}
                    onChange={(e) => setFormData({ ...formData, token: e.target.value })}
                    placeholder='访问令牌，留空则不鉴权'
                    className={inputClass}
                  />
                </div>
                <div className='flex items-center gap-2'>
                  <input
                    type='checkbox'
                    id='enabled'
                    checked={formData.enabled}
                    onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                    className='w-4 h-4 rounded border-gray-300 text-[#165DFF] focus:ring-[#165DFF]'
                  />
                  <label htmlFor='enabled' className='text-sm text-gray-700 dark:text-gray-300'>启用此适配器</label>
                </div>
              </div>
              <div className='flex justify-end gap-2 p-5 border-t border-gray-100 dark:border-gray-800'>
                <button
                  onClick={() => { setShowAddDialog(false); setShowEditDialog(false); resetForm(); }}
                  className='px-4 py-2 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-white/[0.05] rounded-lg transition-colors'
                >
                  取消
                </button>
                <button
                  onClick={showEditDialog ? handleUpdate : handleAdd}
                  disabled={actionLoading !== null}
                  className='px-4 py-2 bg-[#165DFF] text-white dark:bg-white dark:text-black rounded-lg hover:bg-[#0047FF] dark:hover:bg-gray-200 transition-colors flex items-center gap-2 disabled:opacity-50'
                >
                  {actionLoading ? <RefreshCw className='w-4 h-4 animate-spin' /> : <Save className='w-4 h-4' />}
                  {showEditDialog ? '保存' : '添加'}
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}

// --- Logs ---
function LogsTab({ accountId }: { accountId: string }) {
  const [logs, setLogs] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [typeFilter, setTypeFilter] = useState<'all' | 'send' | 'recv' | 'info'>('all');
  const logsContainerRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchLogs = useCallback(async () => {
    setIsLoading(true);
    try {
      const data = await api.getAccountLogs(accountId, 100);
      setLogs(data);
    } catch {}
    setIsLoading(false);
  }, [accountId]);

  useEffect(() => { fetchLogs(); }, [fetchLogs]);

  useEffect(() => {
    let stopped = false;
    const connect = async () => {
      try {
        const url = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/webui/events`;
        const ws = new WebSocket(url);
        wsRef.current = ws;
        ws.onopen = () => { if (reconnectTimerRef.current) { clearTimeout(reconnectTimerRef.current); reconnectTimerRef.current = null; } };
        ws.onmessage = (event) => {
          if (stopped) return;
          try {
            const data = JSON.parse(event.data);
            if (data.type === 'log' && data.account_id === accountId && data.message) {
              setLogs(prev => { const next = [...prev, data.message]; return next.length > 100 ? next.slice(-100) : next; });
            }
          } catch {}
        };
        ws.onclose = () => { if (!stopped) { wsRef.current = null; reconnectTimerRef.current = setTimeout(() => { if (!stopped) connect(); }, 3000); } };
        ws.onerror = () => { ws.close(); };
      } catch {}
    };
    connect();
    return () => { stopped = true; if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current); if (wsRef.current) wsRef.current.close(); };
  }, [accountId]);

  const getLogType = (log: string): 'send' | 'recv' | 'info' => {
    if (log.includes('[send]')) return 'send';
    if (log.includes('[recv]')) return 'recv';
    return 'info';
  };

  const filteredLogs = logs.filter(log => {
    if (typeFilter !== 'all' && getLogType(log) !== typeFilter) return false;
    if (searchKeyword && !log.toLowerCase().includes(searchKeyword.toLowerCase())) return false;
    return true;
  });

  const handleDownload = () => {
    const content = filteredLogs.join('\n');
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `account-${accountId}-logs.txt`;
    a.click();
    URL.revokeObjectURL(url);
    toast.success('日志已下载');
  };

  const typeOptions = [
    { key: 'all' as const, label: '全部' },
    { key: 'send' as const, label: '发送' },
    { key: 'recv' as const, label: '接收' },
    { key: 'info' as const, label: '信息' },
  ];

  return (
    <motion.div className='flex flex-col h-[calc(100vh-16rem)]' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
      <div className='flex flex-col sm:flex-row gap-3 mb-4'>
        <div className='relative flex-1'>
          <Search className='absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500 dark:text-gray-400' />
          <input
            type='text'
            placeholder='搜索日志关键词...'
            value={searchKeyword}
            onChange={e => setSearchKeyword(e.target.value)}
            className='w-full pl-10 pr-4 py-2 bg-gray-50 dark:bg-white/5 border border-gray-100 dark:border-white/[0.06] rounded-xl text-sm outline-none focus:ring-2 focus:ring-[#165DFF] dark:focus:ring-white/20 backdrop-blur-sm transition-all text-gray-900 dark:text-white'
          />
        </div>
        <div className='flex items-center gap-2'>
          {typeOptions.map(opt => (
            <button
              key={opt.key}
              onClick={() => setTypeFilter(opt.key)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                typeFilter === opt.key
                  ? 'bg-[#165DFF] text-white dark:bg-white dark:text-black'
                  : 'bg-gray-100 text-gray-700 dark:bg-white/[0.06] dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-white/[0.10]'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      <div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-sm border border-gray-100 dark:border-gray-800 rounded-2xl overflow-hidden flex-1 flex flex-col'>
        <div className='flex items-center justify-between px-4 py-2 border-b border-gray-100 dark:border-gray-800 bg-gray-50 dark:bg-white/5'>
          <span className='text-xs text-gray-500 dark:text-gray-400'>
            {filteredLogs.length} / {logs.length} 条日志
          </span>
          <div className='flex items-center gap-2'>
            <motion.button
              onClick={fetchLogs}
              disabled={isLoading}
              className='p-1.5 text-gray-500 dark:text-gray-400 hover:text-[#165DFF] dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/[0.05] rounded-lg transition-colors'
              whileHover={{ scale: 1.1 }}
              whileTap={{ scale: 0.9 }}
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
            </motion.button>
            <button
              onClick={handleDownload}
              className='flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-white/[0.06] hover:bg-gray-200 dark:hover:bg-white/[0.10] rounded-lg transition-colors'
            >
              <Download className='w-3.5 h-3.5' />
              下载
            </button>
          </div>
        </div>
        <div ref={logsContainerRef} className='flex-1 overflow-auto p-3 bg-gray-900 dark:bg-black/40 rounded-b-2xl'>
          {isLoading && filteredLogs.length === 0 ? (
            <div className='flex items-center justify-center h-32 text-gray-500 dark:text-gray-400 text-sm'>
              <RefreshCw className='w-5 h-5 animate-spin mr-2' />
              加载中...
            </div>
          ) : filteredLogs.length === 0 ? (
            <div className='flex items-center justify-center h-32 text-gray-500 dark:text-gray-400 text-sm font-mono'>
              {searchKeyword || typeFilter !== 'all' ? '没有匹配的日志' : '暂无日志'}
            </div>
          ) : (
            <pre className='font-mono text-xs text-green-400 whitespace-pre-wrap break-all leading-relaxed'>
              {filteredLogs.map((log, i) => (
                <div key={i} className='py-0.5 hover:bg-white/5 rounded px-1 transition-colors'>
                  {log}
                </div>
              ))}
            </pre>
          )}
        </div>
      </div>
    </motion.div>
  );
}

// --- Remote ---
function RemoteTab({ accountId }: { accountId: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [viewport, setViewport] = useState({ width: 1920, height: 1080 });
  const pollIntervalRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [inputText, setInputText] = useState('');
  const lastClickRef = useRef({ x: 960, y: 540 });

  const takeScreenshot = useCallback(async () => {
    try {
      const blob = await api.getScreenshot(accountId);
      const img = new Image();
      const url = URL.createObjectURL(blob);
      img.onload = () => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        canvas.width = img.width;
        canvas.height = img.height;
        ctx.drawImage(img, 0, 0);
        URL.revokeObjectURL(url);
      };
      img.src = url;
    } catch {}
  }, [accountId]);

  useEffect(() => {
    api.getViewport(accountId).then(setViewport).catch(() => {});
    pollIntervalRef.current = setInterval(takeScreenshot, 1000 / 8);
    setIsConnected(true);
    return () => { if (pollIntervalRef.current) clearInterval(pollIntervalRef.current); };
  }, [accountId, takeScreenshot]);

  const handleCanvasClick = async (e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) * (canvas.width / rect.width);
    const y = (e.clientY - rect.top) * (canvas.height / rect.height);
    lastClickRef.current = { x: Math.round(x), y: Math.round(y) };
    try { await api.click(accountId, Math.round(x), Math.round(y)); } catch {}
  };

  const handleCanvasContextMenu = async (e: React.MouseEvent<HTMLCanvasElement>) => {
    e.preventDefault();
    const canvas = canvasRef.current;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) * (canvas.width / rect.width);
    const y = (e.clientY - rect.top) * (canvas.height / rect.height);
    lastClickRef.current = { x: Math.round(x), y: Math.round(y) };
    try { await api.rightClick(accountId, Math.round(x), Math.round(y)); } catch {}
  };

  const handleCanvasWheel = async (e: React.WheelEvent<HTMLCanvasElement>) => {
    e.preventDefault();
    const { x, y } = lastClickRef.current;
    try { await api.scroll(accountId, x, y, 0, e.deltaY > 0 ? 120 : -120); } catch {}
  };

  const handleTypeSubmit = async () => {
    if (!inputText.trim()) return;
    const { x, y } = lastClickRef.current;
    try { await api.typeText(accountId, x, y, inputText); } catch {}
    setInputText('');
  };

  return (
    <motion.div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl rounded-xl border border-gray-100 dark:border-gray-800 p-5' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
      <div className='flex items-center justify-between mb-4'>
        <h3 className='text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2'>
          <Monitor className='w-4 h-4 text-gray-900 dark:text-white' />
          远程屏幕
        </h3>
        <div className='flex items-center gap-2'>
          <div className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
          <span className='text-sm text-gray-400'>{isConnected ? '已连接' : '未连接'}</span>
        </div>
      </div>
      <div className='relative bg-black rounded-lg overflow-hidden'>
        <canvas
          ref={canvasRef}
          onClick={handleCanvasClick}
          onContextMenu={handleCanvasContextMenu}
          onWheel={handleCanvasWheel}
          className='w-full cursor-crosshair'
          style={{ maxHeight: '600px', objectFit: 'contain' }}
          tabIndex={0}
        />
      </div>
      <div className='flex gap-2 mt-3'>
        <input
          type='text'
          value={inputText}
          onChange={e => setInputText(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); handleTypeSubmit(); } }}
          placeholder='输入文字后回车发送...'
          className='flex-1 px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white rounded-lg border border-gray-200 dark:border-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500'
        />
        <button onClick={handleTypeSubmit} className='px-3 py-1.5 text-sm bg-blue-500 hover:bg-blue-600 text-white rounded-lg transition-colors'>发送</button>
      </div>
      <p className='text-xs text-gray-500 text-center mt-2'>
        点击屏幕定位 | 滚轮滚动 | 输入文字回车发送 | {viewport.width}x{viewport.height}
      </p>
    </motion.div>
  );
}

// --- Debug ---
function DebugTab({ accountId }: { accountId: string }) {
  const [code, setCode] = useState('return document.title');
  const [result, setResult] = useState('');
  const [isExecuting, setIsExecuting] = useState(false);

  const executeCode = async () => {
    setIsExecuting(true);
    try {
      const res = await api.evalJS(accountId, code);
      setResult(JSON.stringify(res, null, 2));
    } catch (err) {
      setResult(`Error: ${(err as Error).message}`);
    } finally {
      setIsExecuting(false);
    }
  };

  const quickCommands = [
    { label: '页面标题', code: 'return document.title' },
    { label: '页面 URL', code: 'return location.href' },
    { label: 'SDK 状态', code: 'return !!(window.__sdkInst && window.__imCtx)' },
    { label: 'User Info', code: 'return window.userInfoStore?.curLoginUserInfo' },
  ];

  return (
    <motion.div className='bg-white dark:bg-[#1D2129] dark:backdrop-blur-xl rounded-xl border border-gray-100 dark:border-gray-800 p-5' initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
      <h3 className='text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2'>
        <Bug className='w-4 h-4 text-gray-900 dark:text-white' />
        JS 调试
      </h3>
      <div className='flex flex-wrap gap-2 mb-4'>
        {quickCommands.map((cmd) => (
          <button
            key={cmd.label}
            onClick={() => setCode(cmd.code)}
            className='px-3 py-1.5 rounded-lg text-xs border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-white/[0.05] transition-colors'
          >
            {cmd.label}
          </button>
        ))}
      </div>
      <textarea
        value={code}
        onChange={(e) => setCode(e.target.value)}
        className={`${inputClass} h-32 resize-none font-mono`}
        placeholder='输入 JavaScript 代码...'
      />
      <div className='flex justify-end mt-3'>
        <button
          onClick={executeCode}
          disabled={isExecuting || !code}
          className='flex items-center px-6 py-2 bg-[#165DFF] text-white rounded-lg text-sm hover:bg-[#0047FF] transition-all disabled:opacity-50'
        >
          {isExecuting ? <Loader2 className='w-4 h-4 mr-2 animate-spin' /> : <Terminal className='w-4 h-4 mr-2' />}
          执行
        </button>
      </div>
      {result && (
        <div className='relative mt-4'>
          <pre className='p-4 rounded-xl bg-gray-950 border border-gray-800 text-green-400 font-mono text-sm overflow-auto max-h-64'>
            {result}
          </pre>
          <button
            onClick={() => navigator.clipboard.writeText(result)}
            className='absolute top-2 right-2 p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-white/10 transition-colors'
          >
            <Copy className='w-4 h-4' />
          </button>
        </div>
      )}
    </motion.div>
  );
}
