import { useState } from 'react';
import { useRouter } from '@/lib/router';
import { useAuthStore } from '@/stores/authStore';
import { Eye, EyeOff, Loader2 } from 'lucide-react';

export function Setup() {
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const { setup, isLoading, error, clearError } = useAuthStore();
  const { navigate } = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!password || !confirmPassword) return;
    if (password !== confirmPassword) return;
    if (password.length < 6) return;

    try {
      await setup(password);
      navigate('/');
    } catch (error) {
      // Error is handled by the store
    }
  };

  const isValid = password.length >= 6 && password === confirmPassword;

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '1rem', background: 'linear-gradient(ellipse at bottom, #1b2735 0%, #090a0f 100%)' }}>
      <div style={{
        width: '100%', maxWidth: '28rem', padding: '2rem',
        background: 'rgba(255,255,255,0.06)', borderRadius: '1rem',
        border: '1px solid rgba(255,255,255,0.1)', backdropFilter: 'blur(20px)',
        boxShadow: '0 8px 32px rgba(0,0,0,0.4)'
      }}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '1.5rem' }}>
          <img src="/webui/logo.jpg" alt="Logo" style={{ width: '4rem', height: '4rem', borderRadius: '1rem', objectFit: 'cover' }} />

          <div style={{ textAlign: 'center' }}>
            <h1 style={{ fontSize: '1.5rem', fontWeight: 700, color: '#fff', margin: '0 0 0.5rem' }}>初始化设置</h1>
            <p style={{ fontSize: '0.875rem', color: 'rgba(255,255,255,0.5)', margin: 0 }}>
              首次使用需设置管理密码，用于登录 WebUI 管理面板
            </p>
          </div>

          <form onSubmit={handleSubmit} style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <div style={{ position: 'relative' }}>
              <input
                type={showPassword ? 'text' : 'password'}
                placeholder="设置密码（至少 6 位）"
                value={password}
                onChange={(e) => { setPassword(e.target.value); clearError(); }}
                disabled={isLoading}
                style={{
                  width: '100%', padding: '0.625rem 2.5rem 0.625rem 0.75rem',
                  background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.12)',
                  borderRadius: '0.5rem', color: '#fff', fontSize: '0.875rem', outline: 'none'
                }}
              />
              <button
                type="button" onClick={() => setShowPassword(!showPassword)}
                style={{ position: 'absolute', right: '0.75rem', top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', color: 'rgba(255,255,255,0.4)', padding: 0 }}
              >
                {showPassword ? <EyeOff style={{ width: '1.25rem', height: '1.25rem' }} /> : <Eye style={{ width: '1.25rem', height: '1.25rem' }} />}
              </button>
            </div>

            <input
              type={showPassword ? 'text' : 'password'}
              placeholder="确认密码"
              value={confirmPassword}
              onChange={(e) => { setConfirmPassword(e.target.value); clearError(); }}
              disabled={isLoading}
              style={{
                width: '100%', padding: '0.625rem 0.75rem',
                background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.12)',
                borderRadius: '0.5rem', color: '#fff', fontSize: '0.875rem', outline: 'none'
              }}
            />

            {password && confirmPassword && password !== confirmPassword && (
              <p style={{ fontSize: '0.875rem', color: '#f87171', textAlign: 'center', margin: 0 }}>两次输入的密码不一致</p>
            )}

            {password && password.length < 6 && (
              <p style={{ fontSize: '0.875rem', color: '#facc15', textAlign: 'center', margin: 0 }}>密码长度至少 6 位</p>
            )}

            {error && (
              <p style={{ fontSize: '0.875rem', color: '#f87171', textAlign: 'center', margin: 0 }}>{error}</p>
            )}

            <button
              type="submit"
              disabled={isLoading || !isValid}
              style={{
                width: '100%', padding: '0.625rem', border: 'none', borderRadius: '0.5rem',
                background: '#165DFF', color: '#fff',
                fontSize: '0.875rem', fontWeight: 500, cursor: isValid ? 'pointer' : 'not-allowed',
                opacity: isValid ? 1 : 0.5, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem'
              }}
            >
              {isLoading && <Loader2 style={{ width: '1rem', height: '1rem', animation: 'spin 1s linear infinite' }} />}
              确认设置
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
