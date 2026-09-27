import React from 'react';
import {
  Trophy,
  Swords,
  Bot,
  Gamepad2,
  Users,
  Activity,
  Settings,
  Radio,
  UserCheck,
  LogOut,
  LogIn,
  UploadCloud,
  Play,
  Cpu,
  Layers,
  Terminal
} from 'lucide-react';
import { Arena, User } from '../types';

interface NavbarProps {
  currentTab: string;
  onSelectTab: (tab: string) => void;
  arenas: Arena[];
  selectedArenaId: number;
  onSelectArena: (id: number) => void;
  user: User | null;
  onOpenUpload: () => void;
  onOpenTriggerMatch: () => void;
  onOpenLogin: () => void;
  onLogout: () => void;
  matchmakerActive: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  currentTab,
  onSelectTab,
  arenas,
  selectedArenaId,
  onSelectArena,
  user,
  onOpenUpload,
  onOpenTriggerMatch,
  onOpenLogin,
  onLogout,
  matchmakerActive,
}) => {
  const tabs = [
    { id: 'leaderboard', label: 'Standings', icon: Trophy },
    { id: 'matches', label: 'Matches', icon: Swords },
    { id: 'bots', label: 'Bots & Ingestion', icon: Bot },
    { id: 'training', label: 'Entrenar', icon: Terminal },
    { id: 'arenas', label: 'Arenas', icon: Gamepad2 },
    { id: 'teams', label: 'Teams', icon: Users },
    { id: 'audit', label: 'Audit Logs', icon: Activity },
    ...(user?.role === 'admin' ? [{ id: 'system', label: 'System Health', icon: Settings }] : []),
  ];

  return (
    <header className="sticky top-0 z-40 bg-white/95 backdrop-blur-md border-b border-slate-200 text-slate-800 shadow-xs">
      {/* Contest Top Color Bar */}
      <div className="h-1 w-full bg-gradient-to-r from-blue-600 via-emerald-500 via-amber-500 to-indigo-600" />

      {/* Main Brand & Actions Bar */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-14 flex items-center justify-between">
        {/* Left: Brand & Arena Selector */}
        <div className="flex items-center gap-6">
          <div
            onClick={() => onSelectTab('leaderboard')}
            className="flex items-center gap-2.5 cursor-pointer font-bold tracking-tight text-lg text-blue-700 hover:text-blue-800 transition group"
          >
            <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center text-white shadow-sm shadow-blue-500/30 group-hover:scale-105 transition-transform">
              <Cpu className="w-5 h-5" />
            </div>
            <span className="font-black tracking-wider text-slate-900 font-mono">AGENTRIX</span>
            <span className="text-[10px] uppercase tracking-wider px-2 py-0.5 rounded-full bg-blue-100 text-blue-800 font-sans font-bold border border-blue-200">
              Arena
            </span>
          </div>

          {/* Arena Switcher Dropdown */}
          <div className="hidden sm:flex items-center gap-2 text-xs bg-slate-100/90 border border-slate-300/80 rounded-lg px-3 py-1.5 shadow-2xs hover:border-slate-400 transition">
            <Layers className="w-3.5 h-3.5 text-blue-600" />
            <span className="text-slate-500 font-medium">Contest Arena:</span>
            <select
              value={selectedArenaId}
              onChange={(e) => onSelectArena(Number(e.target.value))}
              className="bg-transparent text-slate-900 font-bold focus:outline-none cursor-pointer"
            >
              {arenas.map((a) => (
                <option key={a.id} value={a.id} className="bg-white text-slate-900 font-medium">
                  {a.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Right: Quick Action Buttons & User Profile */}
        <div className="flex items-center gap-2.5">
          {/* Matchmaker status beacon */}
          <div
            title={matchmakerActive ? "Continuous Matchmaker active (auto-scheduling ladder matches)" : "Continuous Matchmaker paused"}
            className={`flex items-center gap-2 px-2.5 py-1 rounded-full text-xs font-semibold border shadow-2xs transition ${
              matchmakerActive
                ? 'bg-emerald-50 text-emerald-800 border-emerald-300'
                : 'bg-slate-100 text-slate-600 border-slate-300'
            }`}
          >
            <span className="relative flex h-2 w-2">
              {matchmakerActive && (
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
              )}
              <span className={`relative inline-flex rounded-full h-2 w-2 ${matchmakerActive ? 'bg-emerald-500' : 'bg-slate-400'}`}></span>
            </span>
            <span className="hidden md:inline font-mono text-[11px]">
              {matchmakerActive ? 'Matchmaker: Active' : 'Matchmaker: Idle'}
            </span>
          </div>

          {/* Upload Bot Action Button */}
          <button
            onClick={onOpenUpload}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-bold bg-blue-600 hover:bg-blue-700 text-white transition shadow-sm shadow-blue-500/20 hover:-translate-y-0.5 active:translate-y-0 cursor-pointer"
          >
            <UploadCloud className="w-4 h-4" />
            <span className="hidden sm:inline">Upload Bot</span>
          </button>

          {/* Schedule / Trigger Match Button */}
          <button
            onClick={onOpenTriggerMatch}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-bold bg-emerald-600 hover:bg-emerald-700 text-white transition shadow-sm shadow-emerald-500/20 hover:-translate-y-0.5 active:translate-y-0 cursor-pointer"
          >
            <Play className="w-4 h-4 fill-white" />
            <span className="hidden sm:inline">Schedule Match</span>
          </button>

          <div className="h-5 w-px bg-slate-200 mx-1" />

          {/* User profile */}
          {user ? (
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-2 text-xs text-slate-700 bg-slate-100 px-2.5 py-1 rounded-lg border border-slate-300/80 shadow-2xs">
                <UserCheck className="w-3.5 h-3.5 text-blue-600" />
                <span className="font-bold text-slate-900">{user.username}</span>
                <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-blue-100 text-blue-800 font-bold border border-blue-200">
                  {user.role}
                </span>
              </div>
              <button
                onClick={onLogout}
                title="Log Out"
                className="p-1.5 rounded-lg text-slate-500 hover:text-rose-600 hover:bg-rose-50 transition border border-transparent hover:border-rose-200"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </div>
          ) : (
            <button
              onClick={onOpenLogin}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-bold bg-slate-100 hover:bg-slate-200 text-slate-800 border border-slate-300 transition shadow-2xs"
            >
              <LogIn className="w-3.5 h-3.5 text-blue-600" />
              <span>Log In</span>
            </button>
          )}
        </div>
      </div>

      {/* Navigation Sub-bar (DOMjudge Tab Style) */}
      <nav className="bg-slate-50 border-t border-slate-200 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto flex items-center space-x-1 overflow-x-auto py-1.5">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const active = currentTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => onSelectTab(tab.id)}
                className={`flex items-center gap-2 px-3.5 py-1.5 rounded-md text-xs whitespace-nowrap transition-all duration-150 cursor-pointer ${
                  active
                    ? 'bg-blue-600 text-white font-bold shadow-sm shadow-blue-600/30'
                    : 'text-slate-600 hover:text-blue-700 hover:bg-slate-200/70 font-semibold'
                }`}
              >
                <Icon className={`w-3.5 h-3.5 ${active ? 'text-white' : 'text-slate-500'}`} />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>
      </nav>
    </header>
  );
};
