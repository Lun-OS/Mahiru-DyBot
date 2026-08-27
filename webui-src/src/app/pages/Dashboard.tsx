import { useEffect, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useAccountStore } from '@/stores/accountStore';
import api from '@/services/api';
import {
  Monitor,
  Cpu,
  HardDrive,
  Wifi,
  ArrowUp,
  ArrowDown,
  Users,
  Server,
} from 'lucide-react';

interface SystemInfo {
  os: string;
  platform: string;
  arch: string;
  kernel_version: string;
  hostname: string;
  go_version: string;
  uptime: number;
}

interface CPUInfo {
  model: string;
  cores: number;
  usage_percent: number;
}

interface MemoryInfo {
  total: number;
  used: number;
  free: number;
  usage_percent: number;
}

interface DiskInfo {
  total: number;
  used: number;
  free: number;
  usage_percent: number;
}

interface NetworkInfo {
  bytes_sent: number;
  bytes_recv: number;
  upload_speed: number;
  download_speed: number;
}

interface ProcessInfo {
  pid: number;
  name: string;
  memory_mb: number;
  memory_percent: number;
  cpu_percent: number;
  start_time: string;
  uptime: string;
}

interface SystemResponse {
  system: SystemInfo;
  cpu: CPUInfo;
  memory: MemoryInfo;
  disk: DiskInfo;
  network: NetworkInfo;
  process: ProcessInfo;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}天 ${h}小时`;
  if (h > 0) return `${h}小时 ${m}分钟`;
  return `${m}分钟`;
}

function UsageRing({ percent, label }: { percent: number; label: string }) {
  const r = 54;
  const c = 2 * Math.PI * r;
  const offset = c - (percent / 100) * c;

  return (
    <div className="flex flex-col items-center">
      <svg width="140" height="140" viewBox="0 0 140 140">
        <circle cx="70" cy="70" r={r} fill="none" stroke="rgba(255,255,255,0.1)" strokeWidth="10" />
        <circle
          cx="70" cy="70" r={r} fill="none"
          stroke={percent > 80 ? '#ef4444' : percent > 60 ? '#f59e0b' : '#3b82f6'}
          strokeWidth="10"
          strokeDasharray={c}
          strokeDashoffset={offset}
          strokeLinecap="round"
          transform="rotate(-90 70 70)"
          style={{ transition: 'stroke-dashoffset 0.5s ease' }}
        />
        <text x="70" y="65" textAnchor="middle" fill="white" fontSize="24" fontWeight="bold">
          {percent.toFixed(1)}%
        </text>
        <text x="70" y="85" textAnchor="middle" fill="#9ca3af" fontSize="12">
          {label}
        </text>
      </svg>
    </div>
  );
}

export function Dashboard() {
  const { accounts, fetchAccounts } = useAccountStore();
  const [sysInfo, setSysInfo] = useState<SystemResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const loadSystemInfo = async () => {
    try {
      await fetchAccounts();
      const info = await api.get<SystemResponse>('/system/info');
      setSysInfo(info);
    } catch (error) {
      console.error('Failed to load system info:', error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadSystemInfo();
    const interval = setInterval(loadSystemInfo, 3000);
    return () => clearInterval(interval);
  }, [fetchAccounts]);

  const onlineAccounts = accounts.filter((a) => a.state === 'online').length;
  const totalAccounts = accounts.length;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-400">加载中...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-white">仪表盘</h1>
        <p className="text-gray-400 mt-1">系统运行状态概览</p>
      </div>

      {/* Top row: System info + Account overview + CPU/Memory */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* System Info */}
        <Card className="glass-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">系统信息</CardTitle>
            <Monitor className="h-4 w-4 text-blue-400" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">系统</span>
              <span className="text-white text-sm">{sysInfo?.system.platform || '--'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">内核版本</span>
              <span className="text-white text-sm font-mono">{sysInfo?.system.kernel_version || '--'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">软件版本</span>
              <span className="text-white text-sm">v3.0.0</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">运行时间</span>
              <span className="text-white text-sm">{sysInfo?.system.uptime ? formatUptime(sysInfo.system.uptime) : '--'}</span>
            </div>
          </CardContent>
        </Card>

        {/* Account Overview */}
        <Card className="glass-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">账号概览</CardTitle>
            <Users className="h-4 w-4 text-green-400" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">在线/总计</span>
              <span className="text-white text-sm">
                <span className="text-green-400 font-bold">{onlineAccounts}</span>
                <span className="text-gray-500"> / </span>
                <span>{totalAccounts}</span>
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">进程 PID</span>
              <span className="text-white text-sm font-mono">{sysInfo?.process.pid || '--'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">进程运行时间</span>
              <span className="text-white text-sm">{sysInfo?.process.uptime || '--'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">系统状态</span>
              <Badge variant={onlineAccounts > 0 ? 'success' : 'secondary'}>
                {onlineAccounts > 0 ? '运行中' : '离线'}
              </Badge>
            </div>
          </CardContent>
        </Card>

        {/* CPU + Memory ring charts */}
        <Card className="glass-card lg:col-span-1">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">资源占用</CardTitle>
            <Cpu className="h-4 w-4 text-purple-400" />
          </CardHeader>
          <CardContent>
            <div className="flex justify-around">
              <UsageRing percent={sysInfo?.cpu.usage_percent || 0} label="CPU" />
              <UsageRing percent={sysInfo?.memory.usage_percent || 0} label="内存" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* CPU detail + Memory detail + Disk */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="glass-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">CPU</CardTitle>
            <Cpu className="h-4 w-4 text-blue-400" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">型号</span>
              <span className="text-white text-xs font-mono truncate max-w-[200px]">{sysInfo?.cpu.model || '--'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">核心数</span>
              <span className="text-white text-sm">{sysInfo?.cpu.cores || '--'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">使用率</span>
              <span className="text-white text-sm">{sysInfo?.cpu.usage_percent?.toFixed(1) || '0'}%</span>
            </div>
          </CardContent>
        </Card>

        <Card className="glass-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">内存</CardTitle>
            <HardDrive className="h-4 w-4 text-green-400" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">总量</span>
              <span className="text-white text-sm">{formatBytes(sysInfo?.memory.total || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">已使用</span>
              <span className="text-white text-sm">{formatBytes(sysInfo?.memory.used || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">进程占用</span>
              <span className="text-white text-sm">{sysInfo?.process.memory_mb?.toFixed(1) || '0'} MB</span>
            </div>
          </CardContent>
        </Card>

        <Card className="glass-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-400">磁盘</CardTitle>
            <Server className="h-4 w-4 text-yellow-400" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">总量</span>
              <span className="text-white text-sm">{formatBytes(sysInfo?.disk.total || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">已使用</span>
              <span className="text-white text-sm">{formatBytes(sysInfo?.disk.used || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-400 text-sm">使用率</span>
              <span className="text-white text-sm">{sysInfo?.disk.usage_percent?.toFixed(1) || '0'}%</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Network */}
      <Card className="glass-card">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium text-gray-400">网络速度</CardTitle>
          <Wifi className="h-4 w-4 text-cyan-400" />
        </CardHeader>
        <CardContent>
          <div className="flex justify-around">
            <div className="flex items-center space-x-3">
              <ArrowUp className="h-5 w-5 text-green-400" />
              <div>
                <p className="text-xs text-gray-400">上传</p>
                <p className="text-white font-medium">{formatBytes(sysInfo?.network.upload_speed || 0)}/s</p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <ArrowDown className="h-5 w-5 text-blue-400" />
              <div>
                <p className="text-xs text-gray-400">下载</p>
                <p className="text-white font-medium">{formatBytes(sysInfo?.network.download_speed || 0)}/s</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Account list */}
      <Card className="glass-card">
        <CardHeader>
          <CardTitle className="text-white">账号状态</CardTitle>
        </CardHeader>
        <CardContent>
          {accounts.length === 0 ? (
            <div className="text-center py-8 text-gray-400">
              暂无账号，请先创建账号
            </div>
          ) : (
            <div className="space-y-3">
              {accounts.map((account) => (
                <div
                  key={account.id}
                  className="flex items-center justify-between p-3 rounded-lg bg-white/5 hover:bg-white/10 transition-colors"
                >
                  <div className="flex items-center space-x-3">
                    <div
                      className={`w-3 h-3 rounded-full ${
                        account.state === 'online'
                          ? 'status-online'
                          : account.state === 'starting'
                          ? 'status-starting'
                          : account.state === 'error'
                          ? 'status-error'
                          : 'status-offline'
                      }`}
                    />
                    <div>
                      <p className="text-sm font-medium text-white">{account.name}</p>
                      <p className="text-xs text-gray-400">UID: {account.uid || '--'}</p>
                    </div>
                  </div>
                  <Badge
                    variant={
                      account.state === 'online'
                        ? 'success'
                        : account.state === 'error'
                        ? 'destructive'
                        : 'secondary'
                    }
                  >
                    {account.state === 'online'
                      ? '在线'
                      : account.state === 'starting'
                      ? '启动中'
                      : account.state === 'error'
                      ? '错误'
                      : '离线'}
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
