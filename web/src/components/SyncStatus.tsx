import { formatDateTime } from '../lib/dates';

interface SyncStatusProps {
  lastSyncAt?: string | null;
  status: string;
}

export function SyncStatus(props: SyncStatusProps) {
  if (props.status === 'revoked') {
    return <span class="badge badge-red">Revoked</span>;
  }

  if (!props.lastSyncAt) {
    return <span class="badge badge-gray">Never synced</span>;
  }

  const syncTime = new Date(props.lastSyncAt).getTime();
  const diffMinutes = (Date.now() - syncTime) / (1000 * 60);

  if (diffMinutes < 60) {
    return <span class="badge badge-green">Synced {formatDateTime(props.lastSyncAt)}</span>;
  }
  if (diffMinutes < 120) {
    return <span class="badge badge-yellow">Synced {formatDateTime(props.lastSyncAt)}</span>;
  }
  return <span class="badge badge-red">Stale ({formatDateTime(props.lastSyncAt)})</span>;
}
