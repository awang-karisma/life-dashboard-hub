import { createSignal, onMount, Show } from 'solid-js';
import { A, useNavigate, useSearchParams } from '@solidjs/router';
import {
  IconLayoutDashboard,
  IconLogin,
  IconKey,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { showToast } from '../components/Toast';

interface AuthInfo {
  local: boolean;
  oidc: {
    enabled: boolean;
    display_name?: string;
    login_url?: string;
  };
}

export function Login() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const inviteToken = () => {
    const inv = params.invite;
    return Array.isArray(inv) ? inv[0] : inv;
  };
  const [email, setEmail] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [authInfo, setAuthInfo] = createSignal<AuthInfo>({
    local: true,
    oidc: { enabled: false },
  });

  onMount(async () => {
    if (params.oidc_login === 'success') {
      const token = inviteToken();
      navigate(token ? `/invite/${token}` : '/');
      return;
    }

    try {
      const info = await ApiClient.get<AuthInfo>('/api/v1/auth/info');
      setAuthInfo(info);
    } catch (err) {
      console.error('Failed to load auth info:', err);
    }
  });

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setLoading(true);

    try {
      await ApiClient.post('/api/v1/auth/login', {
        email: email(),
        password: password(),
      });
      showToast('Logged in successfully', 'success');
      const token = inviteToken();
      navigate(token ? `/invite/${token}` : '/');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Login failed', 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="min-h-screen flex items-center justify-center p-4">
      <div class="bento-card max-w-sm w-full p-8 shadow-2xl">
        {/* Brand Header */}
        <div class="flex flex-col items-center mb-6">
          <div class="w-12 h-12 rounded-[14px] bg-gradient-to-br from-indigo-500 to-cyan-500 flex items-center justify-center shadow-lg mb-3">
            <IconLayoutDashboard size={26} color="white" />
          </div>
          <h1 class="text-xl font-extrabold text-text-primary tracking-tight">Life Dashboard</h1>
          <p class="text-xs text-text-muted mt-1">Sign in to your private telemetry hub</p>
        </div>

        {/* Local Sign In */}
        <Show when={authInfo().local}>
          <form onSubmit={handleSubmit} class="flex flex-col gap-4">
            <div>
              <label class="block text-xs font-semibold text-text-muted mb-1">
                Email Address
              </label>
              <input
                type="email"
                required
                placeholder="you@example.com"
                value={email()}
                onInput={(e) => setEmail(e.currentTarget.value)}
                class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
              />
            </div>

            <div>
              <label class="block text-xs font-semibold text-text-muted mb-1">
                Password
              </label>
              <input
                type="password"
                required
                placeholder="••••••••"
                value={password()}
                onInput={(e) => setPassword(e.currentTarget.value)}
                class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
              />
            </div>

            <button
              type="submit"
              disabled={loading()}
              class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-xs transition-all flex items-center justify-center gap-2 cursor-pointer shadow-sm mt-1 disabled:opacity-50"
            >
              <IconLogin size={16} />
              <span>{loading() ? 'Authenticating...' : 'Sign In'}</span>
            </button>
          </form>
        </Show>

        {/* OIDC Sign In */}
        <Show when={authInfo().oidc?.enabled}>
          <div class="mt-4 pt-4 border-t border-border-default">
            <a
              href={authInfo().oidc.login_url || '/api/v1/auth/oidc/login'}
              class="w-full bg-white/5 hover:bg-white/10 text-text-primary py-2.5 rounded-sm font-semibold text-xs border border-border-default transition-all flex items-center justify-center gap-2 no-underline"
            >
              <IconKey size={16} class="text-accent-cyan" />
              <span>Continue with {authInfo().oidc.display_name || 'SSO'}</span>
            </a>
          </div>
        </Show>

        <div class="text-center mt-6 text-xs text-text-muted">
          Need an account?{' '}
          <A
            href={inviteToken() ? `/register?invite=${encodeURIComponent(inviteToken()!)}` : '/register'}
            class="text-primary-light hover:text-primary font-semibold no-underline"
          >
            Create account
          </A>
        </div>
      </div>
    </div>
  );
}
