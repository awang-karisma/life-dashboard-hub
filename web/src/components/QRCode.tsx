import { onMount, createEffect } from 'solid-js';
import QRCodeLib from 'qrcode';

interface QRCodeProps {
  text: string;
  size?: number;
}

export function QRCode(props: QRCodeProps) {
  let canvasRef: HTMLCanvasElement | undefined;

  const render = async () => {
    if (!canvasRef || !props.text) return;
    try {
      await QRCodeLib.toCanvas(canvasRef, props.text, {
        width: props.size || 240,
        margin: 2,
        color: {
          dark: '#0f172a',
          light: '#ffffff',
        },
      });
    } catch (err) {
      console.error('Failed to generate QR code:', err);
    }
  };

  onMount(render);
  createEffect(() => {
    // Re-render if text or size changes
    if (props.text) {
      render();
    }
  });

  return (
    <div class="qr-container" style={{ display: 'inline-block', padding: '0.5rem', background: '#fff', 'border-radius': '0.5rem', border: '1px solid #e2e8f0' }}>
      <canvas ref={canvasRef} />
    </div>
  );
}
