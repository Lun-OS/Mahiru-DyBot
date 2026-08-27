import { useState } from 'react';
import { useRouter } from '@/lib/router';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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

    if (password !== confirmPassword) {
      return;
    }

    if (password.length < 6) {
      return;
    }

    try {
      await setup(password);
      navigate('/');
    } catch (error) {
      // Error is handled by the store
    }
  };

  const isValid = password.length >= 6 && password === confirmPassword;

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      {/* Star field background */}
      <div className="star-field">
        {Array.from({ length: 50 }).map((_, i) => (
          <div
            key={i}
            className="star"
            style={{
              left: `${Math.random() * 100}%`,
              top: `${Math.random() * 100}%`,
              width: `${Math.random() * 2 + 1}px`,
              height: `${Math.random() * 2 + 1}px`,
              animationDuration: `${Math.random() * 3 + 2}s`,
              animationDelay: `${Math.random() * 2}s`,
            }}
          />
        ))}
      </div>

      <Card className="w-full max-w-md glass-card">
        <CardHeader className="space-y-1 text-center">
          <div className="flex justify-center mb-4">
            <img src="/webui/logo.jpg" alt="Logo" className="w-16 h-16 rounded-2xl object-cover" />
          </div>
          <CardTitle className="text-2xl font-bold text-white">初始化设置</CardTitle>
          <CardDescription className="text-gray-400">
            首次使用请设置管理密码
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="relative">
              <Input
                type={showPassword ? 'text' : 'password'}
                placeholder="设置密码（至少 6 位）"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  clearError();
                }}
                className="bg-white/5 border-white/10 text-white placeholder:text-gray-500 pr-10"
                disabled={isLoading}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
              >
                {showPassword ? (
                  <EyeOff className="w-5 h-5" />
                ) : (
                  <Eye className="w-5 h-5" />
                )}
              </button>
            </div>

            <Input
              type={showPassword ? 'text' : 'password'}
              placeholder="确认密码"
              value={confirmPassword}
              onChange={(e) => {
                setConfirmPassword(e.target.value);
                clearError();
              }}
              className="bg-white/5 border-white/10 text-white placeholder:text-gray-500"
              disabled={isLoading}
            />

            {password && confirmPassword && password !== confirmPassword && (
              <p className="text-sm text-red-400 text-center">两次输入的密码不一致</p>
            )}

            {password && password.length < 6 && (
              <p className="text-sm text-yellow-400 text-center">密码长度至少 6 位</p>
            )}

            {error && (
              <p className="text-sm text-red-400 text-center">{error}</p>
            )}

            <Button
              type="submit"
              className="w-full bg-gradient-to-r from-blue-500 to-purple-600 hover:from-blue-600 hover:to-purple-700"
              disabled={isLoading || !isValid}
            >
              {isLoading ? (
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              ) : null}
              确认设置
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
