import type { ReplayData, ReplayTickFrame } from '../types/index.ts';

// Render recorded geometry/rules, not the arena's current configuration.
export function drawReplayLayers(ctx: CanvasRenderingContext2D, replay: ReplayData,
  frame: ReplayTickFrame, width: number, height: number): void {
  const sx = width / (replay.arena?.width ?? 1200);
  const sy = height / (replay.arena?.height ?? 750);
  const rules = (replay.effective_config ?? replay.config)?.match_rules;
  ctx.save();
  ctx.fillStyle = '#334155';
  ctx.strokeStyle = '#94a3b8';
  ctx.lineWidth = 1;
  for (const wall of replay.walls ?? []) {
    ctx.fillRect(wall.x * sx, wall.y * sy, wall.w * sx, wall.h * sy);
    ctx.strokeRect(wall.x * sx, wall.y * sy, wall.w * sx, wall.h * sy);
  }
  if (rules?.zone !== false && frame.zone_radius !== undefined) {
    ctx.strokeStyle = '#f43f5e';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.beginPath();
    ctx.ellipse(width / 2, height / 2, frame.zone_radius * sx, frame.zone_radius * sy, 0, 0, Math.PI * 2);
    ctx.stroke();
  }
  if (rules?.show_vision === true) {
    ctx.setLineDash([3, 3]);
    ctx.strokeStyle = '#38bdf866';
    ctx.lineWidth = 1;
    for (const unit of frame.units) {
      if (!unit.alive || !Number.isFinite(unit.vision) || (unit.vision ?? 0) <= 0) continue;
      ctx.beginPath();
      ctx.ellipse(unit.x * sx, unit.y * sy, unit.vision! * sx, unit.vision! * sy, 0, 0, Math.PI * 2);
      ctx.stroke();
    }
  }
  ctx.restore();
}
