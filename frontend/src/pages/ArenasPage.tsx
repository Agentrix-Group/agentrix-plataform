import React from 'react';
import { Gamepad2, Layers, Users, Clock, ShieldCheck } from 'lucide-react';
import { Arena } from '../types';

interface ArenasPageProps {
  arenas: Arena[];
}

export const ArenasPage: React.FC<ArenasPageProps> = ({ arenas }) => {
  return (
    <div className="space-y-4">
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Gamepad2 className="w-5 h-5 text-sky-400" />
          <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
            Competitive Game Arenas
          </h1>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {arenas.map((arena) => (
          <div
            key={arena.id}
            className="bg-slate-900 border border-slate-800 rounded-lg p-5 shadow-sm hover:border-slate-700 transition space-y-3"
          >
            <div className="flex items-start justify-between">
              <div>
                <h3 className="text-sm font-bold text-slate-100">{arena.name}</h3>
                <span className="font-mono text-[10px] text-sky-400 font-semibold">{arena.slug}</span>
              </div>
              <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-emerald-950/80 text-emerald-400 border border-emerald-800/80">
                ACTIVE
              </span>
            </div>

            <p className="text-xs text-slate-400 leading-relaxed">{arena.description}</p>

            <div className="grid grid-cols-3 gap-2 pt-2 border-t border-slate-800/80 text-xs">
              <div className="bg-slate-950/60 p-2 rounded border border-slate-800/60">
                <span className="text-[10px] text-slate-500 font-medium block">Match Capacity</span>
                <span className="font-mono font-semibold text-slate-200">{arena.max_players} Players</span>
              </div>
              <div className="bg-slate-950/60 p-2 rounded border border-slate-800/60">
                <span className="text-[10px] text-slate-500 font-medium block">Max Duration</span>
                <span className="font-mono font-semibold text-slate-200">{arena.max_ticks} Ticks</span>
              </div>
              <div className="bg-slate-950/60 p-2 rounded border border-slate-800/60">
                <span className="text-[10px] text-slate-500 font-medium block">Game Engine</span>
                <span className="font-mono font-semibold text-sky-400">{arena.game_type}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
