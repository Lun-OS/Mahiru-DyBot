import { useState, useEffect } from 'react';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import api from '@/services/api';
import { toast } from 'sonner';
import { motion } from 'motion/react';
import {
  Key,
  Shield,
  Info,
  Save,
  Loader2,
  Eye,
  EyeOff,
  Wifi,
  CheckCircle,
  Palette,
  Sun,
  Moon,
} from 'lucide-react';
import { useTheme } from 'next-themes';

const inputClass =
  'w-full p-2.5 bg-white/50 dark:bg-white/5 border border-white/40 dark:border-white/10 rounded-lg outline-none focus:ring-2 focus:ring-[#165DFF] dark:focus:ring-white/20 backdrop-blur-sm text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500';

function ConfigPageItem({ children }: { children: React.ReactNode }) {
  return (
    <div className='w-full mx-auto backdrop-blur-sm border border-white/40 dark:border-white/10 shadow-sm rounded-2xl bg-white/60 dark:bg-black/40 dark:backdrop-blur-xl max-w-3xl'>
      <div className='py-6 px-4 md:py-8 md:px-12'>
        <div className='w-full flex flex-col gap-5'>{children}</div>
      </div>
    </div>
  );
}

function SectionTitle({ icon: Icon, title, description }: { icon: React.ElementType; title: string; description?: string }) {
  return (
    <div>
      <div className='flex items-center'>
        <Icon className='w-5 h-5 text-[#165DFF] dark:text-white mr-2' />
        <h2 className='text-lg font-semibold text-gray-900 dark:text-white'>{title}</h2>
      </div>
      {description && <p className='text-sm text-gray-600 dark:text-gray-400 mt-1'>{description}</p>}
    </div>
  );
}

export function SettingsPage() {
  const [activeTab, setActiveTab] = useState('connection');
  const [backendVersion, setBackendVersion] = useState('');

  useEffect(() => { api.getVersion().then((v) => setBackendVersion(v)).catch(() => {}); }, []);

  return (
    <section className='w-full max-w-[1200px] mx-auto py-4 md:py-8 px-2 md:px-6 relative'>
      <Tabs value={activeTab} onValueChange={setActiveTab} className='w-full flex flex-col items-center'>
        <TabsList className='bg-white/40 dark:bg-black/20 backdrop-blur-sm rounded-2xl p-1.5 border border-white/40 dark:border-white/10 mb-4 md:mb-8 w-full md:w-fit mx-auto overflow-x-auto'>
          {[
            { value: 'connection', label: '连接配置', icon: Wifi },
            { value: 'appearance', label: '外观设置', icon: Palette },
            { value: 'security', label: '安全', icon: Shield },
            { value: 'about', label: '关于', icon: Info },
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

        <TabsContent value='connection' className='w-full relative p-0'>
          <ConnectionTab />
        </TabsContent>
        <TabsContent value='appearance' className='w-full relative p-0'>
          <AppearanceTab />
        </TabsContent>
        <TabsContent value='security' className='w-full relative p-0'>
          <SecurityTab />
        </TabsContent>
        <TabsContent value='about' className='w-full relative p-0'>
          <AboutTab backendVersion={backendVersion} />
        </TabsContent>
      </Tabs>
    </section>
  );
}

function ConnectionTab() {
  const [token, setToken] = useState('');
  const [showToken, setShowToken] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api.getGlobalToken().then((data) => setToken(data.token || '')).catch(() => {});
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.setGlobalToken(token.trim());
      toast.success('已保存');
    } catch { toast.error('保存失败'); }
    finally { setSaving(false); }
  };

  return (
    <ConfigPageItem>
      <SectionTitle icon={Key} title='API Token' description='全局访问令牌，未设置账号 Token 时使用此令牌鉴权' />
      <div className='space-y-3'>
        <div className='relative'>
          <input
            type={showToken ? 'text' : 'password'}
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder='留空则不鉴权'
            className={`${inputClass} pr-10 font-mono`}
          />
          <button
            type='button'
            onClick={() => setShowToken(!showToken)}
            className='absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-white'
          >
            {showToken ? <EyeOff className='w-4 h-4' /> : <Eye className='w-4 h-4' />}
          </button>
        </div>
        <p className='text-xs text-gray-500 dark:text-gray-400'>
          连接时需携带: Authorization: Bearer {'<token>'}
        </p>
        <motion.button
          onClick={handleSave}
          disabled={saving}
          className='flex items-center px-6 py-2 bg-[#165DFF] text-white rounded-xl text-sm hover:bg-[#0047FF] transition-all shadow-lg shadow-[#165DFF]/20 disabled:opacity-50'
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
        >
          {saving ? <Loader2 className='w-4 h-4 mr-2 animate-spin' /> : <Save className='w-4 h-4 mr-2' />}
          保存
        </motion.button>
      </div>
    </ConfigPageItem>
  );
}

function AppearanceTab() {
  const { theme, setTheme } = useTheme();
  const isDark = theme === 'dark';

  return (
    <ConfigPageItem>
      <SectionTitle icon={Palette} title='外观设置' description='自定义界面外观' />
      <div className='space-y-4'>
        <div className='flex items-center justify-between p-4 bg-gray-50 dark:bg-white/5 rounded-xl border border-gray-100 dark:border-gray-800'>
          <div className='flex items-center gap-3'>
            {isDark ? <Moon className='w-5 h-5 text-[#165DFF]' /> : <Sun className='w-5 h-5 text-[#165DFF]' />}
            <div>
              <p className='text-sm font-medium text-gray-900 dark:text-white'>主题模式</p>
              <p className='text-xs text-gray-500 dark:text-gray-400'>当前: {isDark ? '深色' : '浅色'}</p>
            </div>
          </div>
          <button
            onClick={() => setTheme(isDark ? 'light' : 'dark')}
            className='px-4 py-2 text-sm font-medium bg-[#165DFF] text-white rounded-lg hover:bg-[#0047FF] transition-colors'
          >
            切换到{isDark ? '浅色' : '深色'}
          </button>
        </div>
      </div>
    </ConfigPageItem>
  );
}

function SecurityTab() {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [saving, setSaving] = useState(false);

  const handleReset = async () => {
    if (!currentPassword) { toast.error('请输入当前密码'); return; }
    if (newPassword.length < 6) { toast.error('新密码至少 6 位'); return; }
    if (newPassword !== confirmPassword) { toast.error('两次密码输入不一致'); return; }
    setSaving(true);
    try {
      const res = await api.resetPassword(currentPassword, newPassword);
      if (res.ok) {
        toast.success('密码修改成功');
        setCurrentPassword('');
        setNewPassword('');
        setConfirmPassword('');
      } else {
        toast.error(res.error || '修改失败');
      }
    } catch { toast.error('网络错误'); }
    finally { setSaving(false); }
  };

  return (
    <ConfigPageItem>
      <SectionTitle icon={Shield} title='修改访问密码' description='修改 WebUI 管理面板的访问密码' />
      <div className='space-y-4'>
        <div>
          <label className='text-sm font-medium text-gray-700 dark:text-gray-300'>当前密码</label>
          <div className='relative mt-2'>
            <input type={showCurrent ? 'text' : 'password'} value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} placeholder='请输入当前密码' className={`${inputClass} pr-10`} />
            <button type='button' onClick={() => setShowCurrent(!showCurrent)} className='absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-white'>
              {showCurrent ? <EyeOff className='w-4 h-4' /> : <Eye className='w-4 h-4' />}
            </button>
          </div>
        </div>
        <div>
          <label className='text-sm font-medium text-gray-700 dark:text-gray-300'>新密码</label>
          <div className='relative mt-2'>
            <input type={showNew ? 'text' : 'password'} value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder='请输入新密码（至少 6 位）' className={`${inputClass} pr-10`} />
            <button type='button' onClick={() => setShowNew(!showNew)} className='absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-white'>
              {showNew ? <EyeOff className='w-4 h-4' /> : <Eye className='w-4 h-4' />}
            </button>
          </div>
        </div>
        <div>
          <label className='text-sm font-medium text-gray-700 dark:text-gray-300'>确认新密码</label>
          <div className='relative mt-2'>
            <input type={showConfirm ? 'text' : 'password'} value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} placeholder='请再次输入新密码' className={`${inputClass} pr-10`} />
            <button type='button' onClick={() => setShowConfirm(!showConfirm)} className='absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-white'>
              {showConfirm ? <EyeOff className='w-4 h-4' /> : <Eye className='w-4 h-4' />}
            </button>
          </div>
        </div>
        <motion.button
          onClick={handleReset}
          disabled={saving}
          className='flex items-center px-6 py-2 bg-[#165DFF] text-white rounded-xl text-sm hover:bg-[#0047FF] transition-all shadow-lg shadow-[#165DFF]/20 disabled:opacity-50'
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
        >
          {saving ? <Loader2 className='w-4 h-4 mr-2 animate-spin' /> : <Save className='w-4 h-4 mr-2' />}
          确认修改
        </motion.button>
      </div>
    </ConfigPageItem>
  );
}

function AboutTab({ backendVersion }: { backendVersion: string }) {
  return (
    <ConfigPageItem>
      <SectionTitle icon={Info} title='关于' description='系统版本和运行状态' />
      <div className='space-y-3'>
        <div className='flex justify-between'>
          <span className='text-gray-500'>版本</span>
          <span className='text-gray-900 dark:text-white font-mono'>v{backendVersion || '1.0.0'}</span>
        </div>
        <div className='flex justify-between'>
          <span className='text-gray-500'>运行端口</span>
          <span className='text-gray-900 dark:text-white'>17836</span>
        </div>
        <div className='flex justify-between'>
          <span className='text-gray-500'>协议</span>
          <span className='text-gray-900 dark:text-white'>OneBot v11</span>
        </div>
        <div className='flex justify-between'>
          <span className='text-gray-500'>状态</span>
          <Badge variant='success'>
            <CheckCircle className='w-3 h-3 mr-1' />
            运行中
          </Badge>
        </div>
      </div>
    </ConfigPageItem>
  );
}
