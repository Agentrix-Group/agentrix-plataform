import React, { useEffect, useRef, useState, useCallback } from 'react';
import {
  Play,
  Pause,
  RotateCcw,
  SkipBack,
  SkipForward,
  FastForward,
  Volume2,
  VolumeX,
  Maximize2
} from 'lucide-react';
import { ReplayData, ReplayTickFrame } from '../types';

interface ReplayViewer2DProps {
  replayData: ReplayData;
  matchId: number;
}

const SEAT_COLORS = [
  '#38bdf8', // Seat 0: Sky Blue
  '#34d399', // Seat 1: Emerald
  '#fbbf24', // Seat 2: Amber
  '#c084fc', // Seat 3: Purple
  '#fb7185', // Seat 4: Rose
];

export const ReplayViewer2D: React.FC<ReplayViewer2DProps> = ({ replayData, matchId }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Extract frames
  const frames: ReplayTickFrame[] = replayData.frames || replayData.ticks || [];
  const totalTicks = frames.length > 0 ? frames.length - 1 : 1;

  const [currentTick, setCurrentTick] = useState<number>(0);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [speed, setSpeed] = useState<number>(1);
  const [killFeed, setKillFeed] = useState<string[]>([]);

  // Animation loop ref
  const animFrameRef = useRef<number | null>(null);
  const lastTimeRef = useRef<number>(performance.now());
  const tickAccRef = useRef<number>(0);

  // Canvas drawing routine
  const renderFrame = useCallback(
    (tickIdx: number) => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      const width = canvas.width;
      const height = canvas.height;

      // 1. Clear background
      ctx.fillStyle = '#090d16'; // Deep space dark
      ctx.fillRect(0, 0, width, height);

      // 2. Draw Grid Lines
      ctx.strokeStyle = '#1e293b';
      ctx.lineWidth = 1;
      const gridSize = 40;
      for (let x = 0; x <= width; x += gridSize) {
        ctx.beginPath();
        ctx.moveTo(x, 0);
        ctx.lineTo(x, height);
        ctx.stroke();
      }
      for (let y = 0; y <= height; y += gridSize) {
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(width, y);
        ctx.stroke();
      }

      if (frames.length === 0 || !frames[tickIdx]) {
        ctx.fillStyle = '#64748b';
        ctx.font = '12px JetBrains Mono';
        ctx.textAlign = 'center';
        ctx.fillText('No frame data available', width / 2, height / 2);
        return;
      }

      const frame = frames[tickIdx];
      const scaleX = width / 1000.0;
      const scaleY = height / 1000.0;

      // 3. Draw Closing Storm Zone
      const zoneRadius = frame.zone_radius !== undefined ? frame.zone_radius * scaleX : 460 * scaleX;
      ctx.save();
      ctx.strokeStyle = '#f43f5e';
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 4]);
      ctx.beginPath();
      ctx.arc(width / 2, height / 2, Math.max(10, zoneRadius), 0, Math.PI * 2);
      ctx.stroke();
      ctx.restore();

      // 4. Draw Projectiles
      if (frame.projectiles) {
        ctx.fillStyle = '#fde047';
        for (const p of frame.projectiles) {
          const px = p.x * scaleX;
          const py = p.y * scaleY;
          ctx.beginPath();
          ctx.arc(px, py, 2.5, 0, Math.PI * 2);
          ctx.fill();
        }
      }

      // 5. Draw Mobs
      if (frame.mobs) {
        ctx.fillStyle = '#ef4444';
        for (const m of frame.mobs) {
          if (!m.alive) continue;
          const mx = m.x * scaleX;
          const my = m.y * scaleY;
          ctx.beginPath();
          ctx.arc(mx, my, 4, 0, Math.PI * 2);
          ctx.fill();
        }
      }

      // 6. Draw Units / Bots
      if (frame.units) {
        for (const u of frame.units) {
          if (!u.alive) continue;

          const ux = u.x * scaleX;
          const uy = u.y * scaleY;
          const seatColor = SEAT_COLORS[u.seat % SEAT_COLORS.length];

          // Bot Body Circle
          ctx.save();
          ctx.shadowColor = seatColor;
          ctx.shadowBlur = 10;
          ctx.fillStyle = seatColor;
          ctx.beginPath();
          ctx.arc(ux, uy, 9, 0, Math.PI * 2);
          ctx.fill();
          ctx.restore();

          // Bot Facing Direction Pointer
          const angle = u.angle || 0;
          const dirX = ux + Math.cos(angle) * 16;
          const dirY = uy + Math.sin(angle) * 16;
          ctx.strokeStyle = '#ffffff';
          ctx.lineWidth = 2;
          ctx.beginPath();
          ctx.moveTo(ux, uy);
          ctx.lineTo(dirX, dirY);
          ctx.stroke();

          // Health Bar Background & Fill
          const barWidth = 24;
          const barHeight = 3;
          const barX = ux - barWidth / 2;
          const barY = uy - 16;
          const maxHp = u.max_hp || 100;
          const hpRatio = Math.max(0, Math.min(1, u.hp / maxHp));

          ctx.fillStyle = '#0f172a';
          ctx.fillRect(barX - 1, barY - 1, barWidth + 2, barHeight + 2);

          ctx.fillStyle = hpRatio > 0.5 ? '#10b981' : hpRatio > 0.25 ? '#f59e0b' : '#ef4444';
          ctx.fillRect(barX, barY, barWidth * hpRatio, barHeight);

          // Seat Number / Label
          ctx.fillStyle = '#f8fafc';
          ctx.font = '9px JetBrains Mono';
          ctx.textAlign = 'center';
          ctx.fillText(`P${u.seat}`, ux, uy + 20);
        }
      }

      // 7. Update Kill Feed
      if (frame.events && frame.events.length > 0) {
        setKillFeed((prev) => [...prev, ...frame.events!].slice(-6));
      }
    },
    [frames]
  );

  // Playback Loop
  useEffect(() => {
    if (!isPlaying) {
      if (animFrameRef.current) {
        cancelAnimationFrame(animFrameRef.current);
      }
      return;
    }

    lastTimeRef.current = performance.now();

    const loop = (now: number) => {
      const deltaMs = now - lastTimeRef.current;
      lastTimeRef.current = now;

      // 50ms per tick at 1x speed
      const tickDuration = 50 / speed;
      tickAccRef.current += deltaMs;

      while (tickAccRef.current >= tickDuration) {
        tickAccRef.current -= tickDuration;
        setCurrentTick((prev) => {
          if (prev >= totalTicks) {
            setIsPlaying(false);
            return totalTicks;
          }
          return prev + 1;
        });
      }

      animFrameRef.current = requestAnimationFrame(loop);
    };

    animFrameRef.current = requestAnimationFrame(loop);

    return () => {
      if (animFrameRef.current) {
        cancelAnimationFrame(animFrameRef.current);
      }
    };
  }, [isPlaying, speed, totalTicks]);

  // Re-render when currentTick changes
  useEffect(() => {
    renderFrame(currentTick);
  }, [currentTick, renderFrame]);

  const handleScrubberChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = Number(e.target.value);
    setCurrentTick(val);
  };

  const togglePlay = () => {
    if (currentTick >= totalTicks) {
      setCurrentTick(0);
    }
    setIsPlaying(!isPlaying);
  };

  const handleStep = (direction: number) => {
    setIsPlaying(false);
    setCurrentTick((prev) => Math.max(0, Math.min(totalTicks, prev + direction)));
  };

  const handleReset = () => {
    setIsPlaying(false);
    setCurrentTick(0);
  };

  return (
    <div ref={containerRef} className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-2xl">
      {/* Top Banner with Match Meta & Live Killfeed */}
      <div className="flex items-center justify-between px-4 py-2.5 bg-slate-950/80 border-b border-slate-800 text-xs">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-slate-200">Replay Viewer</span>
            <span className="font-mono text-sky-400">#Match-{matchId}</span>
          </div>

          <div className="h-3 w-px bg-slate-800" />

          {/* Seat Color Legends */}
          <div className="hidden sm:flex items-center gap-3 text-[11px] font-mono">
            {SEAT_COLORS.map((color, idx) => (
              <div key={idx} className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: color }} />
                <span className="text-slate-400">P{idx}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Current Tick & Time indicator */}
        <div className="font-mono text-[11px] text-slate-400">
          Tick: <span className="text-sky-400 font-semibold">{currentTick}</span> / {totalTicks}
          <span className="text-slate-600 ml-1.5">({((currentTick * 50) / 1000).toFixed(1)}s)</span>
        </div>
      </div>

      {/* Main 2D Canvas Stage */}
      <div className="relative aspect-video w-full bg-slate-950 flex items-center justify-center overflow-hidden">
        <canvas
          ref={canvasRef}
          width={800}
          height={450}
          className="w-full h-full object-contain cursor-crosshair"
        />

        {/* Floating Kill Feed overlay */}
        {killFeed.length > 0 && (
          <div className="absolute top-3 right-3 pointer-events-none space-y-1 text-right max-w-xs">
            {killFeed.map((evt, idx) => (
              <div
                key={idx}
                className="inline-block px-2.5 py-1 rounded bg-slate-900/90 border border-slate-700/80 text-[10px] font-mono text-slate-200 shadow-md backdrop-blur-sm"
              >
                {evt}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Control Toolbar */}
      <div className="px-4 py-3 bg-slate-950/90 border-t border-slate-800 space-y-2">
        {/* Scrubber Timeline */}
        <div className="flex items-center gap-3">
          <input
            type="range"
            min={0}
            max={totalTicks}
            value={currentTick}
            onChange={handleScrubberChange}
            className="w-full h-1.5 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-sky-500 focus:outline-none"
          />
        </div>

        {/* Playback Buttons */}
        <div className="flex items-center justify-between text-xs pt-1">
          <div className="flex items-center gap-2">
            <button
              onClick={handleReset}
              title="Restart"
              className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition"
            >
              <RotateCcw className="w-4 h-4" />
            </button>

            <button
              onClick={() => handleStep(-1)}
              title="Previous Tick"
              className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition"
            >
              <SkipBack className="w-4 h-4" />
            </button>

            <button
              onClick={togglePlay}
              className="flex items-center justify-center w-8 h-8 rounded-full bg-sky-600 hover:bg-sky-500 text-white shadow transition"
            >
              {isPlaying ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4 ml-0.5" />}
            </button>

            <button
              onClick={() => handleStep(1)}
              title="Next Tick"
              className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition"
            >
              <SkipForward className="w-4 h-4" />
            </button>

            <div className="h-4 w-px bg-slate-800 mx-1" />

            {/* Speed Multipliers */}
            <div className="flex items-center gap-1 font-mono text-[11px]">
              {[0.5, 1, 2, 5, 10].map((spd) => (
                <button
                  key={spd}
                  onClick={() => setSpeed(spd)}
                  className={`px-2 py-0.5 rounded transition ${
                    speed === spd
                      ? 'bg-sky-600 text-white font-bold'
                      : 'bg-slate-800/80 text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {spd}x
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-3 text-slate-400 text-[11px] font-mono">
            <span>60 FPS Replay</span>
          </div>
        </div>
      </div>
    </div>
  );
};
