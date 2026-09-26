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
  Layers
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
    { id: 'arenas', label: 'Arenas', icon: Gamepad2 },
    { id: 'teams', label: 'Teams', icon: Users },
    { id: 'audit', label: 'Audit Logs', icon: Activity },
    ...(user?.role === 'admin' ? [{ id: 'system', label: 'System Health', icon: Settings }] : []),
  ];

  return (
    <header className="sticky top-0 z-40 bg-slate-900/95 backdrop-blur border-b border-slate-800 text-slate-200">
      {/* Top Banner Bar */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-14 flex items-center justify-between">
        {/* Left: Brand & Arena Selector */}
        <div className="flex items-center gap-6">
          <div
            onClick={() => onSelectTab('leaderboard')}
            className="flex items-center gap-2 cursor-pointer font-mono font-bold tracking-wider text-base text-sky-400 hover:text-sky-300 transition"
          >
            <div className="w-8 h-8 rounded bg-sky-500/10 border border-sky-500/30 flex items-center justify-center text-sky-400">
              <Cpu className="w-5 h-5" />
            </div>
            <span>AGENTRIX</span>
            <span className="text-[10px] uppercase tracking-widest px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-slate-400 font-sans font-semibold">
              Arena
            </span>
          </div>

          {/* Arena Switcher Dropdown */}
          <div className="hidden sm:flex items-center gap-2 text-xs bg-slate-950/60 border border-slate-800 rounded px-2.5 py-1">
            <Layers className="w-3.5 h-3.5 text-slate-400" />
            <span className="text-slate-400">Arena:</span>
            <select
              value={selectedArenaId}
              onChange={(e) => onSelectArena(Number(e.target.value))}
              className="bg-transparent text-slate-200 font-medium focus:outline-none cursor-pointer"
            >
              {arenas.map((a) => (
                <option key={a.id} value={a.id} className="bg-slate-900 text-slate-200">
                  {a.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Right: Quick Action Buttons & User Profile */}
        <div className="flex items-center gap-3">
          {/* Matchmaker status beacon */}
          <div
            title={matchmakerActive ? "Continuous Matchmaker active (auto-scheduling ladder matches)" : "Continuous Matchmaker paused"}
            className="flex items-center gap-1.5 px-2 py-1 rounded bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-400"
          >
            <Radio className={`w-3.5 h-3.5 ${matchmakerActive ? 'text-emerald-400 animate-pulse' : 'text-slate-500'}`} />
            <span className="hidden md:inline">{matchmakerActive ? 'Ladder: Active' : 'Ladder: Idle'}</span>
          </div>

          {/* Upload Bot Action Button */}
          <button
            onClick={onOpenUpload}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-semibold bg-sky-600 hover:bg-sky-500 text-white transition shadow-sm"
          >
            <UploadCloud className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Upload Bot</span>
          </button>

          {/* Schedule / Trigger Match Button */}
          <button
            onClick={onOpenTriggerMatch}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-semibold bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
          >
            <Play className="w-3.5 h-3.5 text-emerald-400" />
            <span className="hidden sm:inline">Trigger Match</span>
          </button>

          <div className="h-5 w-px bg-slate-800 mx-1" />

          {/* User profile */}
          {user ? (
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-300 bg-slate-950 px-2.5 py-1 rounded border border-slate-800">
                <UserCheck className="w-3.5 h-3.5 text-sky-400" />
                <span className="font-medium">{user.username}</span>
                <span className="text-[10px] uppercase font-mono px-1 py-0.2 rounded bg-slate-800 text-slate-400">
                  {user.role}
                </span>
              </div>
              <button
                onClick={onLogout}
                title="Log Out"
                className="p-1.5 rounded text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </div>
          ) : (
            <button
              onClick={onOpenLogin}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-semibold bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
            >
              <LogIn className="w-3.5 h-3.5 text-sky-400" />
              <span>Log In</span>
            </button>
          )}
        </div>
      </div>

      {/* Navigation Sub-bar (DOMjudge Tab Style) */}
      <nav className="bg-slate-950/80 border-t border-slate-800/80 px-4 sm:px-6 lg:px-8">
        <div className="max-w-7xl mx-auto flex items-center space-x-1 overflow-x-auto py-1">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const active = currentTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => onSelectTab(tab.id)}
                className={`flex items-center gap-2 px-3 py-1.5 rounded text-xs font-medium whitespace-nowrap transition ${
                  active
                    ? 'bg-slate-800 text-sky-400 border border-slate-700 shadow-sm font-semibold'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
                }`}
              >
                <Icon className={`w-3.5 h-3.5 ${active ? 'text-sky-400' : 'text-slate-400'}`} />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>
      </nav>
    </header>
  );
};
