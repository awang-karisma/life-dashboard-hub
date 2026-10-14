import { createSignal, onMount, Show } from 'solid-js';
import { A, useNavigate, useParams } from '@solidjs/router';
import {
  IconUsers,
  IconCheck,
  IconAlertCircle,
  IconLogin,
  IconUserPlus,
} from '@tabler/icons-solidjs';
import { showToast } from '../components/Toast';

interface InvitationPreview {
  family_name: string;
  invited_email: string;
  role: string;
  expires_at: string;
}

interface CurrentUser {
  id: number;
  email: string;
  display_name: string;
}

export function AcceptInvite() {
  const params = useParams<{ token: string }>();
  const navigate = useNavigate();

  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);
  const [isExpiredOrInvalid, setIsExpiredOrInvalid] = createSignal(false);
  const [invite, setInvite] = createSignal<InvitationPreview | null>(null);
  const [currentUser, setCurrentUser] = createSignal<CurrentUser | null>(null);
  const [accepting, setAccepting] = createSignal(false);
  const [acceptError, setAcceptError] = createSignal<string | null>(null);
  const [alreadyMember, setAlreadyMember] = createSignal(false);

  onMount(async () => {
    const token = params.token;
    if (!token) {
      setError('Missing invitation token');
      setLoading(false);
      return;
    }

    try {
      const previewRes = await fetch(`/api/v1/invitations/${encodeURIComponent(token)}/preview`, {
        headers: { Accept: 'application/json' },
      });

      if (previewRes.status === 404 || previewRes.status === 410) {
        setIsExpiredOrInvalid(true);
        const errJson = await previewRes.json().catch(() => ({}));
        setError(errJson.error || 'Invalid or expired invitation');
        setLoading(false);
        return;
      }

      if (!previewRes.ok) {
        const errJson = await previewRes.json().catch(() => ({}));
        setError(errJson.error || 'Failed to load invitation details');
        setLoading(false);
        return;
      }

      const previewData = (await previewRes.json()) as InvitationPreview;
      setInvite(previewData);
    } catch {
      setError('Unable to connect to server. Please check your connection.');
      setLoading(false);
      return;
    }

    // Check if already authenticated using raw fetch (ApiClient redirects to /login on 401)
    try {
      const authRes = await fetch('/api/v1/users/me', {
        credentials: 'same-origin',
        headers: { Accept: 'application/json' },
      });
      if (authRes.ok) {
        const user = (await authRes.json()) as CurrentUser;
        setCurrentUser(user);
      }
    } catch {
      // User is not authenticated
    } finally {
      setLoading(false);
    }
  });

  const handleAccept = async () => {
    const token = params.token;
    if (!token) return;

    setAccepting(true);
    setAcceptError(null);
    setAlreadyMember(false);

    try {
      const res = await fetch(`/api/v1/invitations/${encodeURIComponent(token)}/accept`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { Accept: 'application/json' },
      });

      if (res.ok) {
        showToast('Joined family!', 'success');
        navigate('/family');
        return;
      }

      const errJson = await res.json().catch(() => ({}));
      if (res.status === 403) {
        setAcceptError('This invitation was sent to a different email address.');
      } else if (res.status === 409) {
        setAlreadyMember(true);
        setAcceptError(errJson.error || 'You already belong to a family.');
      } else {
        setAcceptError(errJson.error || 'Failed to accept invitation');
      }
    } catch (err: unknown) {
      setAcceptError(err instanceof Error ? err.message : 'Network error occurred');
    } finally {
      setAccepting(false);
    }
  };

  return (
    <div class="min-h-screen flex items-center justify-center p-4">
      <div class="bento-card max-w-sm w-full p-8 shadow-2xl">
        {/* Header */}
        <div class="flex flex-col items-center mb-6">
          <div class="w-12 h-12 rounded-[14px] bg-gradient-to-br from-indigo-500 to-cyan-500 flex items-center justify-center shadow-lg mb-3">
            <IconUsers size={26} color="white" />
          </div>
          <h1 class="text-xl font-extrabold text-text-primary tracking-tight">Family Invitation</h1>
          <p class="text-xs text-text-muted mt-1">Join your household on Life Dashboard</p>
        </div>

        {/* Loading */}
        <Show when={loading()}>
          <div class="py-12 text-center text-text-muted flex flex-col items-center justify-center gap-2">
            <span class="inline-block w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
            <span class="text-sm font-medium">Loading invitation...</span>
          </div>
        </Show>

        {/* Error */}
        <Show when={!loading() && error()}>
          <div class="flex flex-col items-center text-center gap-4 py-2">
            <div class="w-12 h-12 rounded-full bg-red-500/10 text-red-400 flex items-center justify-center">
              <IconAlertCircle size={28} />
            </div>
            <div>
              <h2 class="text-base font-bold text-text-primary mb-1">
                {isExpiredOrInvalid() ? 'Invitation Expired or Invalid' : 'Unable to Load Invitation'}
              </h2>
              <p class="text-xs text-text-muted">{error()}</p>
            </div>
            <A
              href="/login"
              class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-xs text-center transition-all no-underline mt-2 cursor-pointer shadow-sm"
            >
              Return to Sign In
            </A>
          </div>
        </Show>

        {/* Content */}
        <Show when={!loading() && !error() && invite()}>
          <div class="bg-bg-card border border-border-default rounded-md p-4 mb-6 flex flex-col gap-2.5">
            <div class="flex justify-between items-center text-xs">
              <span class="text-text-muted">Family</span>
              <span class="font-bold text-text-primary text-sm">{invite()?.family_name}</span>
            </div>
            <div class="flex justify-between items-center text-xs">
              <span class="text-text-muted">Role</span>
              <span class="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-primary/20 text-primary-light uppercase tracking-wider">
                {invite()?.role}
              </span>
            </div>
            <Show when={invite()?.invited_email}>
              <div class="flex justify-between items-center text-xs">
                <span class="text-text-muted">Sent to</span>
                <span class="text-text-primary font-medium">{invite()?.invited_email}</span>
              </div>
            </Show>
          </div>

          {/* Authenticated State */}
          <Show when={currentUser()}>
            <div class="flex flex-col gap-3">
              <p class="text-xs text-text-muted text-center mb-1">
                Accept invitation to join <strong class="text-text-primary">{invite()?.family_name}</strong>?
              </p>
              <button
                type="button"
                onClick={handleAccept}
                disabled={accepting()}
                class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-xs transition-all flex items-center justify-center gap-2 cursor-pointer shadow-sm disabled:opacity-50"
              >
                <IconCheck size={16} />
                <span>{accepting() ? 'Joining...' : 'Accept Invitation'}</span>
              </button>

              <Show when={acceptError()}>
                <div class="p-3 rounded bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
                  <p>{acceptError()}</p>
                  <Show when={alreadyMember()}>
                    <A href="/family" class="text-primary-light hover:underline font-semibold block mt-1.5">
                      Go to your family &rarr;
                    </A>
                  </Show>
                </div>
              </Show>

              <p class="text-[11px] text-text-muted text-center mt-2">
                Signed in as <span class="text-text-primary font-medium">{currentUser()?.email}</span>
              </p>
            </div>
          </Show>

          {/* Unauthenticated State */}
          <Show when={!currentUser()}>
            <div class="flex flex-col gap-3">
              <p class="text-xs text-text-muted text-center mb-2">
                Sign in or create an account to accept this invitation.
              </p>
              <A
                href={`/register?invite=${encodeURIComponent(params.token)}`}
                class="w-full bg-primary hover:bg-primary-hover text-white py-2.5 rounded-sm font-semibold text-xs transition-all flex items-center justify-center gap-2 no-underline shadow-sm"
              >
                <IconUserPlus size={16} />
                <span>Create Account</span>
              </A>
              <A
                href={`/login?invite=${encodeURIComponent(params.token)}`}
                class="w-full bg-white/5 hover:bg-white/10 text-text-primary py-2.5 rounded-sm font-semibold text-xs border border-border-default transition-all flex items-center justify-center gap-2 no-underline"
              >
                <IconLogin size={16} />
                <span>Sign In</span>
              </A>
            </div>
          </Show>
        </Show>
      </div>
    </div>
  );
}
