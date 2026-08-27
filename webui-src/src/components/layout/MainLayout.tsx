import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from '@/lib/router';
import { useAuthStore } from '@/stores/authStore';
import { useTheme } from 'next-themes';
import { AnimatePresence, motion } from 'motion/react';
import { Menu, PanelLeftClose, Sun, Moon, LogOut, LayoutDashboard, Users, Settings as SettingsIcon } from 'lucide-react';
import { toast } from 'sonner';

function StarFieldBackground() {
  const stars = useMemo(
    () =>
      [...Array(50)].map((_, i) => {
        const type = Math.random();
        const baseOpacity = 0.25 + Math.random() * 0.5;
        return {
          id: i,
          x: Math.random() * 100,
          y: Math.random() * 100,
          size: Math.random() * 2 + 0.3,
          type,
          baseOpacity,
          duration: type > 0.8 ? 0.8 + Math.random() * 1 : 2 + Math.random() * 4,
          delay: Math.random() * 6,
        };
      }),
    []
  );

  return (
    <div className='absolute inset-0 overflow-hidden pointer-events-none dark:block hidden'>
      <style>{`
        @keyframes star-twinkle-fast {
          0%, 100% { opacity: var(--base-op); transform: scale(1); }
          50% { opacity: 0.03; transform: scale(0.5); }
        }
        @keyframes star-twinkle-slow {
          0%, 100% { opacity: var(--base-op); }
          33% { opacity: calc(var(--base-op) * 0.2); }
          66% { opacity: calc(var(--base-op) * 0.5); }
        }
        @keyframes star-breathe-glow {
          0%, 100% { opacity: var(--base-op); transform: scale(1); box-shadow: 0 0 2px rgba(255,255,255,0.15); }
          50% { opacity: 0.95; transform: scale(1.35); box-shadow: 0 0 8px rgba(255,255,255,0.7), 0 0 16px rgba(255,255,255,0.25); }
        }
      `}</style>
      <div className='absolute inset-0 bg-gradient-to-b from-[#050810] via-[#0a0f18] to-[#050810]' />
      <div
        className='absolute top-[-10%] left-[-10%] w-[500px] h-[500px] rounded-full blur-[130px]'
        style={{ background: 'radial-gradient(circle, rgba(20,25,35,0.7) 0%, transparent 70%)', animation: 'pulse-glow 14s ease-in-out infinite' }}
      />
      <div
        className='absolute bottom-[-10%] right-[-10%] w-[450px] h-[450px] rounded-full blur-[130px]'
        style={{ background: 'radial-gradient(circle, rgba(15,20,28,0.65) 0%, transparent 70%)', animation: 'pulse-glow-reverse 18s ease-in-out infinite' }}
      />
      <style>{`
        @keyframes pulse-glow { 0%, 100% { transform: scale(1); opacity: 0.4; } 50% { transform: scale(1.12); opacity: 0.8; } }
        @keyframes pulse-glow-reverse { 0%, 100% { transform: scale(1.1); opacity: 0.5; } 50% { transform: scale(1); opacity: 0.85; } }
      `}</style>
      {stars.map((star) => (
        <div
          key={star.id}
          className='absolute rounded-full bg-white'
          style={{
            left: `${star.x}%`,
            top: `${star.y}%`,
            width: `${star.size}px`,
            height: `${star.size}px`,
            ['--base-op' as string]: star.baseOpacity,
            boxShadow: star.size > 1 ? '0 0 3px rgba(255,255,255,0.4), 0 0 6px rgba(255,255,255,0.15)' : 'none',
            animationName: star.type > 0.8 ? 'star-twinkle-fast' : star.type > 0.5 ? 'star-breathe-glow' : 'star-twinkle-slow',
            animationDuration: `${star.duration}s`,
            animationDelay: `${star.delay}s`,
            animationTimingFunction: 'ease-in-out',
            animationIterationCount: 'infinite',
            willChange: 'opacity, transform',
          }}
        />
      ))}
    </div>
  );
}

const menuItems = [
  { label: '首页', path: '/' },
  { label: '账号列表', path: '/accounts' },
  { label: '系统设置', path: '/settings' },
];

const findTitle = (pathname: string): string[] => {
  for (const item of menuItems) {
    if (item.path === pathname) return [item.label];
  }
  if (pathname.startsWith('/accounts/')) return ['账号详情'];
  return [];
};

function Sidebar({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { path, navigate } = useRouter();
  const { theme, setTheme } = useTheme();
  const logout = useAuthStore((s) => s.logout);
  const isDark = theme === 'dark';
  const toggleTheme = () => setTheme(isDark ? 'light' : 'dark');

  const handleLogout = () => {
    logout();
    toast.success('退出登录成功');
    navigate('/login');
  };

  const menuIcons: Record<string, React.ElementType> = {
    '/': LayoutDashboard,
    '/accounts': Users,
    '/settings': SettingsIcon,
  };

  const isActive = (itemPath: string) => {
    if (itemPath === '/') return path === '/';
    return path.startsWith(itemPath);
  };

  return (
    <>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            className='fixed inset-y-0 left-64 right-0 bg-black/20 backdrop-blur-[1px] z-40 md:hidden'
            aria-hidden='true'
            onClick={onClose}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0, transition: { duration: 0.15 } }}
            transition={{ duration: 0.2, delay: 0.15 }}
          />
        )}
      </AnimatePresence>
      <motion.div
        className='overflow-hidden fixed top-0 left-0 h-full z-50 md:static md:shadow-none rounded-r-2xl md:rounded-none bg-white/80 backdrop-blur-xl backdrop-saturate-150 shadow-xl dark:bg-black/40 dark:border-r dark:border-white/[0.06] md:bg-transparent md:backdrop-blur-none md:backdrop-saturate-100 md:shadow-none'
        initial={{ width: 0 }}
        animate={{ width: open ? '16rem' : 0 }}
        transition={{ type: open ? 'spring' : 'tween', stiffness: 150, damping: open ? 15 : 10 }}
        style={{ overflow: 'hidden' }}
      >
        <div className='w-64 flex flex-col items-stretch h-full transition-transform duration-300 ease-in-out z-30 relative float-right p-4'>
          <div className='flex items-center justify-start gap-3 px-2 my-8 ml-2'>
            <div className='h-5 w-1 bg-[#165DFF] dark:bg-white/60 rounded-full shadow-sm' />
            <div className='text-xl font-bold tracking-wide select-none text-gray-900 dark:text-white'>
              Mahiru DyBot
            </div>
          </div>
          <div className='overflow-y-auto flex flex-col flex-1 px-2'>
            <nav className='flex flex-col gap-2'>
              {menuItems.map((item) => {
                const active = isActive(item.path);
                const Icon = menuIcons[item.path];
                return (
                  <button
                    key={item.path}
                    onClick={() => { navigate(item.path); onClose(); }}
                    className={`flex items-center w-full text-left justify-start transition-all duration-300 rounded-lg px-3 py-2.5 text-sm ${
                      active
                        ? 'bg-[#165DFF]/10 text-[#165DFF] dark:bg-white/10 dark:text-white font-semibold translate-x-1'
                        : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-white/[0.06] hover:translate-x-1'
                    }`}
                  >
                    {Icon && <Icon className='w-5 h-5 mr-3 shrink-0 text-gray-500 dark:text-gray-500' />}
                    <span className='flex-1'>{item.label}</span>
                    <div className={`w-3 h-1.5 rounded-full ml-auto transition-all ${
                      active ? 'bg-[#165DFF] dark:bg-white/70' : 'bg-transparent dark:bg-white/10'
                    }`} />
                  </button>
                );
              })}
            </nav>
            <div className='mt-auto mb-10 md:mb-0 space-y-3 px-2'>
              <button
                className='w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-full text-sm font-medium bg-[#165DFF]/10 hover:bg-[#165DFF]/20 text-[#165DFF] shadow-sm hover:shadow-md transition-all duration-300 backdrop-blur-sm dark:bg-white/[0.06] dark:hover:bg-white/[0.12] dark:text-gray-300 dark:shadow-none'
                onClick={toggleTheme}
              >
                {isDark ? <Sun className='w-4 h-4' /> : <Moon className='w-4 h-4' />}
                切换主题
              </button>
              <button
                className='w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-full text-sm font-medium bg-red-50/50 hover:bg-red-100/80 dark:bg-red-500/10 dark:hover:bg-red-500/20 text-red-500 shadow-sm hover:shadow-md transition-all duration-300 backdrop-blur-sm'
                onClick={handleLogout}
              >
                <LogOut className='w-4 h-4' />
                退出登录
              </button>
            </div>
          </div>
        </div>
      </motion.div>
    </>
  );
}

export function MainLayout({ children }: { children: React.ReactNode }) {
  const { path } = useRouter();
  const [openSideBar, setOpenSideBar] = useState(true);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    contentRef.current?.scrollTo?.({ top: 0, behavior: 'smooth' });
  }, [path]);

  const title = useMemo(() => findTitle(path), [path]);

  return (
    <div className='h-screen relative flex items-stretch overflow-hidden bg-gray-50 dark:bg-[#050810]'>
      <StarFieldBackground />
      <Sidebar open={openSideBar} onClose={() => setOpenSideBar(false)} />
      <motion.div
        layout
        ref={contentRef}
        initial={{ opacity: 0, scale: 0.98 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.4 }}
        className='flex-1 flex flex-col overflow-hidden transition-all duration-300 ease-in-out relative z-10'
      >
        <div className='h-10 flex items-center font-bold text-xl backdrop-blur-lg rounded-full bg-white/70 dark:bg-black/30 shadow-sm dark:shadow-none border border-gray-200/50 dark:border-white/[0.06] m-2 mb-0 flex-shrink-0'>
          <div className={`mr-1 ease-in-out ml-0 md:relative z-50 md:z-auto ${openSideBar ? 'pl-2' : ''} md:!ml-0 md:pl-0`}>
            <button
              className='p-2 rounded-full hover:bg-gray-100 dark:hover:bg-white/[0.08] transition-colors'
              onClick={() => setOpenSideBar(!openSideBar)}
            >
              {openSideBar ? (
                <PanelLeftClose className='w-5 h-5 text-gray-700 dark:text-gray-400' />
              ) : (
                <Menu className='w-5 h-5 text-gray-700 dark:text-gray-400' />
              )}
            </button>
          </div>
          <div className='flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400'>
            <AnimatePresence mode='wait'>
              <motion.div
                key={title.join('')}
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 10 }}
                transition={{ duration: 0.3 }}
                className='text-gray-900 dark:text-white font-medium'
              >
                {title.join('')}
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
        <AnimatePresence mode='wait'>
          <motion.div
            key={path}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.2 }}
            className='flex-1 min-h-0 overflow-y-auto'
          >
            {children}
          </motion.div>
        </AnimatePresence>
      </motion.div>
    </div>
  );
}
