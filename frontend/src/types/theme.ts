/**
 * Canonical Seat Color Definition for Agentrix Arena.
 * Defines immutable, unified color mappings across:
 * - 2D Canvas simulation units and direction pointers
 * - Live HUD participant cards
 * - Match Detail Scoreboard table
 * - Kill feeds and match logs
 */

export interface SeatColorConfig {
  seat: number;
  name: string;
  hex: string;
  glow: string;
  bgClass: string;
  textClass: string;
  borderClass: string;
  badgeClass: string;
}

export const SEAT_COLORS: SeatColorConfig[] = [
  {
    seat: 0,
    name: 'Sky Blue',
    hex: '#38bdf8', // sky-400
    glow: 'rgba(56, 189, 248, 0.4)',
    bgClass: 'bg-sky-500 text-white',
    textClass: 'text-sky-600',
    borderClass: 'border-sky-400',
    badgeClass: 'bg-sky-100 text-sky-800 border-sky-300',
  },
  {
    seat: 1,
    name: 'Emerald Green',
    hex: '#34d399', // emerald-400
    glow: 'rgba(52, 211, 153, 0.4)',
    bgClass: 'bg-emerald-500 text-white',
    textClass: 'text-emerald-600',
    borderClass: 'border-emerald-400',
    badgeClass: 'bg-emerald-100 text-emerald-800 border-emerald-300',
  },
  {
    seat: 2,
    name: 'Amber Yellow',
    hex: '#fbbf24', // amber-400
    glow: 'rgba(251, 191, 36, 0.4)',
    bgClass: 'bg-amber-500 text-white',
    textClass: 'text-amber-600',
    borderClass: 'border-amber-400',
    badgeClass: 'bg-amber-100 text-amber-800 border-amber-300',
  },
  {
    seat: 3,
    name: 'Purple Violet',
    hex: '#c084fc', // purple-400
    glow: 'rgba(192, 132, 252, 0.4)',
    bgClass: 'bg-purple-500 text-white',
    textClass: 'text-purple-600',
    borderClass: 'border-purple-400',
    badgeClass: 'bg-purple-100 text-purple-800 border-purple-300',
  },
  {
    seat: 4,
    name: 'Rose Red',
    hex: '#fb7185', // rose-400
    glow: 'rgba(251, 113, 133, 0.4)',
    bgClass: 'bg-rose-500 text-white',
    textClass: 'text-rose-600',
    borderClass: 'border-rose-400',
    badgeClass: 'bg-rose-100 text-rose-800 border-rose-300',
  },
];

export const SEAT_HEX_ARRAY = SEAT_COLORS.map((c) => c.hex);

export function getSeatColor(seat: number): SeatColorConfig {
  const normalized = ((seat % SEAT_COLORS.length) + SEAT_COLORS.length) % SEAT_COLORS.length;
  return SEAT_COLORS[normalized];
}

export interface EntityColorConfig {
  hex: string;
  bgClass: string;
  textClass: string;
  borderClass: string;
  badgeClass: string;
}

export const ENTITY_PALETTE: EntityColorConfig[] = [
  {
    hex: '#38bdf8',
    bgClass: 'bg-sky-500 text-white',
    textClass: 'text-sky-600',
    borderClass: 'border-sky-300',
    badgeClass: 'bg-sky-100 text-sky-800 border-sky-300',
  },
  {
    hex: '#34d399',
    bgClass: 'bg-emerald-500 text-white',
    textClass: 'text-emerald-600',
    borderClass: 'border-emerald-300',
    badgeClass: 'bg-emerald-100 text-emerald-800 border-emerald-300',
  },
  {
    hex: '#fbbf24',
    bgClass: 'bg-amber-500 text-white',
    textClass: 'text-amber-600',
    borderClass: 'border-amber-300',
    badgeClass: 'bg-amber-100 text-amber-800 border-amber-300',
  },
  {
    hex: '#c084fc',
    bgClass: 'bg-purple-500 text-white',
    textClass: 'text-purple-600',
    borderClass: 'border-purple-300',
    badgeClass: 'bg-purple-100 text-purple-800 border-purple-300',
  },
  {
    hex: '#fb7185',
    bgClass: 'bg-rose-500 text-white',
    textClass: 'text-rose-600',
    borderClass: 'border-rose-300',
    badgeClass: 'bg-rose-100 text-rose-800 border-rose-300',
  },
  {
    hex: '#818cf8',
    bgClass: 'bg-indigo-500 text-white',
    textClass: 'text-indigo-600',
    borderClass: 'border-indigo-300',
    badgeClass: 'bg-indigo-100 text-indigo-800 border-indigo-300',
  },
  {
    hex: '#2dd4bf',
    bgClass: 'bg-teal-500 text-white',
    textClass: 'text-teal-600',
    borderClass: 'border-teal-300',
    badgeClass: 'bg-teal-100 text-teal-800 border-teal-300',
  },
  {
    hex: '#fb923c',
    bgClass: 'bg-orange-500 text-white',
    textClass: 'text-orange-600',
    borderClass: 'border-orange-300',
    badgeClass: 'bg-orange-100 text-orange-800 border-orange-300',
  },
];

/**
 * Deterministic hash-based color for entities without a fixed seat
 * (e.g. Teams, Global Bots in rosters, Leaderboard items).
 * Ensures that Bot "Hunter" or Team "MIT" ALWAYS has the same color everywhere,
 * invariant across re-ordering, pagination, or filtering.
 */
export function getDeterministicColor(identifier: string | number): EntityColorConfig {
  const str = String(identifier);
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = (hash << 5) - hash + str.charCodeAt(i);
    hash |= 0;
  }
  const idx = Math.abs(hash) % ENTITY_PALETTE.length;
  return ENTITY_PALETTE[idx];
}
