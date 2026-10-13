import { createSignal } from 'solid-js';
import { A, useNavigate } from '@solidjs/router';
import {
  IconLayoutDashboard,
  IconUserPlus,
} from '@tabler/icons-solidjs';
import { ApiClient } from '../api/client';
import { showToast } from '../components/Toast';

export function Register() {
  const navigate = useNavigate();
  const [email, setEmail] = createSignal('');
  const [displayName, setDisplayName] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [loading, setLoading] = createSignal(false);

  const handleSubmit = async (e: Event) => {
    e.preventDefault();

    if (password().length < 8) {
      showToast('Password must be at least 8 characters', 'error');
      return;
    }

    setLoading(true);
    try {
      await ApiClient.post('/api/v1/auth/register', {
        email: email(),
        password: password(),
        display_name: displayName(),
      });

      // Automatically log in after registration
      await ApiClient.post('/api/v1/auth/login', {
        email: email(),
        password: password(),
      });

      showToast('Account created and signed in', 'success');
      navigate('/');
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : 'Registration failed', 'error');
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
          <h1 class="text-xl font-extrabold text-text-primary tracking-tight">Create Account</h1>
          <p class="text-xs text-text-muted mt-1">Initialize your private Life Dashboard instance</p>
        </div>

        <form onSubmit={handleSubmit} class="flex flex-col gap-4">
          <div>
            <label class="block text-xs font-semibold text-text-muted mb-1">
              Display Name
            </label>
            <input
              type="text"
              required
              placeholder="Your Name"
              value={displayName()}
              onInput={(e) => setDisplayName(e.currentTarget.value)}
              class="w-full bg-bg-card border border-border-default rounded-sm px-3 py-2 text-sm text-text-primary focus:border-border-focus focus:outline-none"
            />
          </div>

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
              Password (min 8 characters)
            </label>
            <input
              type="password"
              required
              minLength={8}
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
            <IconUserPlus size={16} />
            <span>{loading() ? 'Creating account...' : 'Create Account'}</span>
          </button>
        </form>

        <div class="text-center mt-6 text-xs text-text-muted">
          Already have an account?{' '}
          <A href="/login" class="text-primary-light hover:text-primary font-semibold no-underline">
            Sign in
          </A>
        </div>
      </div>
    </div>
  );
}
