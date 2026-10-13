import { JSX } from 'solid-js';

export function PulseDot(props: { color?: string }): JSX.Element {
  return (
    <span
      class="pulse-dot inline-block"
      style={props.color ? { 'background-color': props.color, 'box-shadow': `0 0 8px ${props.color}` } : undefined}
    />
  );
}
