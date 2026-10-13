import { formatDateTime } from '../lib/dates';
import { PulseDot } from './PulseDot';

interface SyncStatusProps {
  lastSyncAt?: string | null;
  status: string;
}

export function SyncStatus(props: SyncStatusProps) {
  if (props.status === 'revoked') {
    return (
      <span class="rounded-full px-2.5 py-1 text-xs font-semibold inline-flex items-center gap-1.5 bg-danger-bg text-danger">
        Revoked
      </span>
    );
  }

  if (!props.lastSyncAt) {
    return (
      <span class="rounded-full px-2.5 py-1 text-xs font-semibold inline-flex items-center gap-1.5 bg-white/5 text-text-muted">
        Never synced
      </span>
    );
  }

  const syncTime = new Date(props.lastSyncAt).getTime();
  const diffMinutes = (Date.now() - syncTime) / (1000 * 60);

  if (diffMinutes < 60) {
    return (
      <span class="rounded-full px-2.5 py-1 text-xs font-semibold inline-flex items-center gap-1.5 bg-success-bg text-success">
        <PulseDot />
        <span>Synced {formatDateTime(props.lastSyncAt)}</span>
      </span>
    );
  }
  if (diffMinutes < 120) {
    return (
      <span class="rounded-full px-2.5 py-1 text-xs font-semibold inline-flex items-center gap-1.5 bg-warning-bg text-warning">
        Synced {formatDateTime(props.lastSyncAt)}
      </span>
    );
  }
  return (
    <span class="rounded-full px-2.5 py-1 text-xs font-semibold inline-flex items-center gap-1.5 bg-danger-bg text-danger">
      Stale ({formatDateTime(props.lastSyncAt)})
    </span>
  );
}
