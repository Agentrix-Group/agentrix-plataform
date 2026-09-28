import React, { useEffect, useRef, useState, useCallback } from 'react';
import { drawReplayLayers } from '../api/replayLayers';
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
import { ReplayData, ReplayTickFrame, MatchParticipant, getSeatColor, SEAT_COLORS } from '../types';

interface ReplayViewer2DProps {
  replayData: ReplayData;
  matchId: number;
  participants?: MatchParticipant[];
}

export const ReplayViewer2D: React.FC<ReplayViewer2DProps> = ({ replayData, matchId, participants }) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Extract frames
  const frames: ReplayTickFrame[] = replayData.frames || (Array.isArray(replayData.ticks) ? replayData.ticks : []);
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
      const scaleX = width / (replayData.arena?.width ?? 1200);
      const scaleY = height / (replayData.arena?.height ?? 750);

      // 3. Draw Closing Storm Zone
      drawReplayLayers(ctx, replayData, frame, width, height);

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
          const seatColor = getSeatColor(u.seat).hex;

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

          // Seat Number / Bot Name Label
          const participant = participants?.find((p) => p.seat === u.seat);
          const botLabel = participant?.agent_name
            ? (participant.agent_name.length > 9 ? participant.agent_name.slice(0, 8) + '…' : participant.agent_name)
            : `P${u.seat}`;

          ctx.fillStyle = '#f8fafc';
          ctx.font = '9px JetBrains Mono, monospace';
          ctx.textAlign = 'center';
          ctx.fillText(botLabel, ux, uy + 20);
        }
      }

      // 7. Update Kill Feed
      setKillFeed(frame.kill_feed ?? frame.events?.slice(-6) ?? []);
    },
    [frames, replayData, participants]
  );

  const tickHz = replayData.arena?.tick_hz || 60;

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

      // Real-time tick duration based on engine tick_hz (e.g. 60 Hz = ~16.67ms)
      const tickDuration = (1000 / tickHz) / speed;
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
  }, [isPlaying, speed, totalTicks, tickHz]);

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

  const currentFrame = frames[currentTick] || null;
  const currentUnits = currentFrame?.units || [];

  return (
    <div ref={containerRef} className="bg-white border-2 border-slate-200/90 rounded-2xl overflow-hidden shadow-md space-y-0">
      {/* Top Banner with Match Meta & Live Killfeed */}
      <div className="flex flex-wrap items-center justify-between px-5 py-3 bg-slate-50 border-b border-slate-200 text-xs">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <span className="font-extrabold text-slate-800 text-sm">Arena Simulation</span>
            <span className="font-mono text-blue-700 bg-blue-50 border border-blue-200 px-2 py-0.5 rounded-full font-bold">
              Match #{matchId}
            </span>
          </div>

          <div className="h-4 w-px bg-slate-300 hidden sm:block" />

          {/* Seat Color Legends */}
          <div className="hidden sm:flex items-center gap-3 text-xs font-mono font-bold">
            {participants && participants.length > 0 ? (
              participants.map((p) => {
                const theme = getSeatColor(p.seat);
                return (
                  <div key={p.seat} className="flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-white border border-slate-200 shadow-2xs">
                    <span className="w-2.5 h-2.5 rounded-full shadow-xs" style={{ backgroundColor: theme.hex }} />
                    <span className="text-slate-700 font-semibold">{p.agent_name || `Seat ${p.seat}`}</span>
                  </div>
                );
              })
            ) : (
              SEAT_COLORS.map((theme, idx) => (
                <div key={idx} className="flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-white border border-slate-200 shadow-2xs">
                  <span className="w-2.5 h-2.5 rounded-full shadow-xs" style={{ backgroundColor: theme.hex }} />
                  <span className="text-slate-700">Seat {idx}</span>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Current Tick & Time indicator */}
        <div className="font-mono text-xs font-bold text-slate-600 bg-white border border-slate-200 px-3 py-1 rounded-lg shadow-2xs">
          Tick: <span className="text-blue-600 font-extrabold">{currentTick}</span> / {totalTicks}
          <span className="text-slate-400 font-medium ml-1.5">({(currentTick / tickHz).toFixed(1)}s)</span>
        </div>
      </div>

      {/* Main 2D Canvas Stage */}
      <div style={{ aspectRatio: `${replayData.arena?.width ?? 1200} / ${replayData.arena?.height ?? 750}` }} className="relative w-full bg-[#0a1128] flex items-center justify-center overflow-hidden">
        <canvas
          ref={canvasRef}
          width={800}
          height={Math.round(800 * (replayData.arena?.height ?? 750) / (replayData.arena?.width ?? 1200))}
          className="w-full h-full object-contain cursor-crosshair"
        />

        {/* Floating Kill Feed overlay */}
        {killFeed.length > 0 && (
          <div className="absolute top-4 right-4 pointer-events-none space-y-1.5 text-right max-w-sm">
            {killFeed.map((evt, idx) => (
              <div
                key={idx}
                className="inline-block px-3 py-1.5 rounded-lg bg-white/95 border border-slate-300/80 text-xs font-bold text-slate-900 shadow-lg backdrop-blur-md animate-fade-in"
              >
                ⚔ {evt}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Live Participant Status HUD Strip */}
      <div className="px-5 py-3 bg-slate-50 border-t border-b border-slate-200">
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-2.5">
          {(participants && participants.length > 0
            ? [...participants].sort((a, b) => a.seat - b.seat)
            : SEAT_COLORS.map((_, idx) => ({ seat: idx, agent_name: `Seat ${idx}` }))
          ).map((p) => {
            const seatIdx = p.seat;
            const theme = getSeatColor(seatIdx);
            const unit = currentUnits.find((u) => u.seat === seatIdx);
            const isAlive = unit ? unit.alive : true;
            const maxHp = unit?.max_hp || 100;
            const hp = unit ? unit.hp : 100;
            const hpPct = Math.max(0, Math.min(100, (hp / maxHp) * 100));

            return (
              <div
                key={seatIdx}
                className={`p-2.5 rounded-xl border transition-all ${
                  isAlive
                    ? 'bg-white border-slate-200/90 shadow-2xs'
                    : 'bg-slate-100 border-slate-200/60 opacity-60'
                }`}
              >
                <div className="flex items-center justify-between text-xs font-mono mb-1.5">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ backgroundColor: theme.hex }} />
                    <span className="font-extrabold text-slate-800 truncate" title={p.agent_name}>
                      {p.agent_name}
                    </span>
                    <span className="text-[10px] text-slate-400 font-bold shrink-0">P{seatIdx}</span>
                  </div>
                  <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded shrink-0 ${
                    isAlive ? 'bg-emerald-50 text-emerald-700' : 'bg-rose-50 text-rose-700 line-through'
                  }`}>
                    {isAlive ? `${Math.round(hp)} HP` : 'DEAD'}
                  </span>
                </div>
                {/* Health bar */}
                <div className="w-full bg-slate-200 rounded-full h-1.5 overflow-hidden">
                  <div
                    className={`h-1.5 rounded-full transition-all duration-100 ${
                      hpPct > 50 ? 'bg-emerald-500' : hpPct > 20 ? 'bg-amber-500' : 'bg-rose-500'
                    }`}
                    style={{ width: `${isAlive ? hpPct : 0}%` }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Control Toolbar */}
      <div className="px-5 py-4 bg-white space-y-3">
        {/* Scrubber Timeline */}
        <div className="flex items-center gap-3">
          <input
            type="range"
            min={0}
            max={totalTicks}
            value={currentTick}
            onChange={handleScrubberChange}
            className="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-blue-600 focus:outline-none"
          />
        </div>

        {/* Playback Buttons */}
        <div className="flex flex-wrap items-center justify-between gap-3 text-xs pt-1">
          <div className="flex items-center gap-2">
            <button
              onClick={handleReset}
              title="Restart"
              className="p-2 rounded-lg hover:bg-slate-100 text-slate-600 hover:text-slate-900 transition border border-slate-200 hover:border-slate-300 shadow-2xs cursor-pointer"
            >
              <RotateCcw className="w-4 h-4" />
            </button>

            <button
              onClick={() => handleStep(-1)}
              title="Previous Tick"
              className="p-2 rounded-lg hover:bg-slate-100 text-slate-600 hover:text-slate-900 transition border border-slate-200 hover:border-slate-300 shadow-2xs cursor-pointer"
            >
              <SkipBack className="w-4 h-4" />
            </button>

            <button
              onClick={togglePlay}
              className="flex items-center justify-center w-10 h-10 rounded-full bg-blue-600 hover:bg-blue-700 text-white shadow-md shadow-blue-500/30 hover:scale-105 active:scale-95 transition-all cursor-pointer"
            >
              {isPlaying ? <Pause className="w-5 h-5 fill-white" /> : <Play className="w-5 h-5 ml-0.5 fill-white" />}
            </button>

            <button
              onClick={() => handleStep(1)}
              title="Next Tick"
              className="p-2 rounded-lg hover:bg-slate-100 text-slate-600 hover:text-slate-900 transition border border-slate-200 hover:border-slate-300 shadow-2xs cursor-pointer"
            >
              <SkipForward className="w-4 h-4" />
            </button>

            <div className="h-5 w-px bg-slate-200 mx-2 hidden sm:block" />

            {/* Speed Multipliers */}
            <div className="flex items-center gap-1.5 font-mono text-xs font-bold">
              {[0.5, 1, 2, 5, 10].map((spd) => (
                <button
                  key={spd}
                  onClick={() => setSpeed(spd)}
                  className={`px-2.5 py-1 rounded-md transition-all cursor-pointer ${
                    speed === spd
                      ? 'bg-blue-600 text-white shadow-xs'
                      : 'bg-slate-100 text-slate-600 hover:bg-slate-200 hover:text-slate-900'
                  }`}
                >
                  {spd}x
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-2 text-slate-500 text-xs font-mono font-semibold">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
            <span>60 FPS Tactical Replay Engine</span>
          </div>
        </div>
      </div>
    </div>
  );
};
