export function formatMinutes(totalMinutes: number): string {
  if (!totalMinutes || totalMinutes <= 0) return '0m';
  const hours = Math.floor(totalMinutes / 60);
  const mins = Math.round(totalMinutes % 60);
  if (hours === 0) return `${mins}m`;
  if (mins === 0) return `${hours}h`;
  return `${hours}h ${mins}m`;
}

export function formatSteps(steps: number): string {
  return (steps || 0).toLocaleString();
}

export function formatDistance(meters: number): string {
  if (!meters || meters <= 0) return '0 km';
  const km = meters / 1000;
  return `${km.toFixed(1)} km`;
}

export function formatCalories(kcal: number): string {
  return `${Math.round(kcal || 0)} kcal`;
}
