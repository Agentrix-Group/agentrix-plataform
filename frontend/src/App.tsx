import React, { useEffect, useState, useCallback } from 'react';
import { Navbar } from './components/Navbar';
import { LeaderboardPage } from './pages/LeaderboardPage';
import { MatchesPage } from './pages/MatchesPage';
import { MatchDetailPage } from './pages/MatchDetailPage';
import { BotsPage } from './pages/BotsPage';
import { ArenasPage } from './pages/ArenasPage';
import { TeamsPage } from './pages/TeamsPage';
import { AuditLogsPage } from './pages/AuditLogsPage';
import { SystemStatusPage } from './pages/SystemStatusPage';
import { UploadBotModal } from './components/UploadBotModal';
import { TriggerMatchModal } from './components/TriggerMatchModal';
import { LoginPage } from './pages/LoginPage';
import { api } from './api/client';
import { Arena, LadderEntry, Match, AgentVersion, Team, AuditLog, SystemStatus, User } from './types';
import { Cpu, ShieldCheck } from 'lucide-react';

export const App: React.FC = () => {
  const [currentTab, setCurrentTab] = useState<string>('leaderboard');
  const [selectedMatchId, setSelectedMatchId] = useState<number | null>(null);

  // Core Data State
  const [arenas, setArenas] = useState<Arena[]>([]);
  const [selectedArenaId, setSelectedArenaId] = useState<number>(1);
  const [ladder, setLadder] = useState<LadderEntry[]>([]);
  const [matches, setMatches] = useState<Match[]>([]);
  const [agents, setAgents] = useState<AgentVersion[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [user, setUser] = useState<User | null>(null);

  const [loading, setLoading] = useState<boolean>(false);

  // Modals
  const [isUploadOpen, setIsUploadOpen] = useState<boolean>(false);
  const [isTriggerMatchOpen, setIsTriggerMatchOpen] = useState<boolean>(false);
  const [isLoginOpen, setIsLoginOpen] = useState<boolean>(false);

  // Load Initial Arenas & Current User
  useEffect(() => {
    api.getArenas().then((res) => {
      setArenas(res);
      if (res.length > 0) {
        setSelectedArenaId(res[0].id);
      }
    }).catch(console.error);

    api.getMe().then(setUser).catch(() => {
      // not logged in
    });

    api.getSystemStatus().then(setSystemStatus).catch(console.error);
  }, []);

  // Fetch Ladder & Matches whenever selectedArenaId or tab changes
  const refreshData = useCallback(async () => {
    if (!selectedArenaId) return;
    setLoading(true);

    try {
      const [ladderRes, matchesRes, agentsRes, teamsRes, arenasRes] = await Promise.all([
        api.getLadder(selectedArenaId),
        api.getMatches(selectedArenaId),
        api.getAgents(selectedArenaId),
        api.getTeams(),
        api.getArenas(),
      ]);

      setLadder(ladderRes || []);
      setMatches(matchesRes || []);
      setAgents(agentsRes || []);
      setTeams(teamsRes || []);
	  setArenas(arenasRes || []);

      if (currentTab === 'audit') {
        const logsRes = await api.getAuditLogs();
        setAuditLogs(logsRes || []);
      }

      if (currentTab === 'system') {
        const sysRes = await api.getSystemStatus();
        setSystemStatus(sysRes);
      }
    } catch (err) {
      console.error('Failed to refresh data', err);
    } finally {
      setLoading(false);
    }
  }, [selectedArenaId, currentTab, user]);

  useEffect(() => {
    refreshData();
  }, [refreshData]);

  // Periodic poll for continuous ladder updates
  useEffect(() => {
    const timer = setInterval(() => {
      refreshData();
    }, 15000); // 15 seconds
    return () => clearInterval(timer);
  }, [refreshData]);

  const handleSelectMatch = (matchId: number) => {
    setSelectedMatchId(matchId);
    setCurrentTab('match-detail');
  };

  const handleRerunMatch = async (matchId: number) => {
    try {
      const res = await api.rerunMatch(matchId);
      handleSelectMatch(res.new_match_id);
    } catch (err: any) {
      alert(err.message || 'Error re-running match');
    }
  };

  const selectedArena = arenas.find((a) => a.id === selectedArenaId) || null;

  return (
    <div className="min-h-screen bg-slate-100/70 text-slate-900 flex flex-col font-sans">
      {/* Top Navbar */}
      <Navbar
        currentTab={currentTab}
        onSelectTab={(tab) => {
          setSelectedMatchId(null);
          setCurrentTab(tab);
        }}
        arenas={arenas}
        selectedArenaId={selectedArenaId}
        onSelectArena={setSelectedArenaId}
        user={user}
        onOpenUpload={() => setIsUploadOpen(true)}
        onOpenTriggerMatch={() => setIsTriggerMatchOpen(true)}
        onOpenLogin={() => setIsLoginOpen(true)}
        onLogout={() => {
          api.logout();
          setUser(null);
          setLadder([]); setMatches([]); setAuditLogs([]);
          setSelectedMatchId(null); setCurrentTab('leaderboard');
        }}
        matchmakerActive={systemStatus?.matchmaker_active ?? false}
      />

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
        {currentTab === 'leaderboard' && (
          <LeaderboardPage
            ladder={ladder}
            arena={selectedArena}
            loading={loading}
            onRefresh={refreshData}
            onTriggerMatch={() => setIsTriggerMatchOpen(true)}
            onSelectMatch={handleSelectMatch}
          />
        )}

        {currentTab === 'matches' && (
          <MatchesPage
            key={`${selectedArenaId}-${user?.id ?? 'public'}-${user?.role ?? 'public'}-${selectedArena?.frozen ?? false}`}
            arenaId={selectedArenaId}
            matches={matches}
            arenas={arenas}
            loading={loading}
            onRefresh={refreshData}
            onSelectMatch={handleSelectMatch}
            onRerunMatch={handleRerunMatch}
            onTriggerMatch={() => setIsTriggerMatchOpen(true)}
          />
        )}

        {currentTab === 'match-detail' && selectedMatchId && (
          <MatchDetailPage
            key={`${user?.id ?? 'public'}-${user?.role ?? 'public'}-${selectedArena?.frozen ?? false}`}
            matchId={selectedMatchId}
            onBack={() => setCurrentTab('matches')}
            onRerun={handleRerunMatch}
          />
        )}

        {currentTab === 'bots' && (
          <BotsPage
            agents={agents}
            arenas={arenas}
            user={user}
            loading={loading}
            onRefresh={refreshData}
            onOpenUpload={() => setIsUploadOpen(true)}
          />
        )}

        {currentTab === 'arenas' && <ArenasPage arenas={arenas} canFreeze={user?.role === 'admin'} onFreeze={async (id, frozen) => { await api.setArenaFreeze(id, frozen); await refreshData(); }} />}

        {currentTab === 'teams' && <TeamsPage teams={teams} />}

        {currentTab === 'audit' && (
          <AuditLogsPage
            logs={auditLogs}
            loading={loading}
            onRefresh={refreshData}
          />
        )}

        {currentTab === 'system' && (
          <SystemStatusPage
            status={systemStatus}
            loading={loading}
            onRefresh={refreshData}
          />
        )}
      </main>

      {/* Footer */}
      <footer className="bg-white border-t border-slate-200 py-4 text-xs text-slate-500 shadow-2xs">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Cpu className="w-4 h-4 text-blue-600" />
            <span className="font-bold text-slate-700">Agentrix Arena Platform</span>
            <span className="text-slate-400">• Multi-Agent Continuous Benchmark Ladder</span>
          </div>
          <div className="flex items-center gap-4 text-[11px] font-mono text-slate-500">
            <span className="px-2 py-0.5 rounded bg-slate-100 border border-slate-200 text-slate-700 font-semibold">Rust Arbiter v1.0</span>
            <span className="px-2 py-0.5 rounded bg-slate-100 border border-slate-200 text-slate-700 font-semibold">Go Monolith API</span>
            <span className="px-2 py-0.5 rounded bg-slate-100 border border-slate-200 text-slate-700 font-semibold">React + TypeScript SPA</span>
          </div>
        </div>
      </footer>

      {/* Modals */}
      <UploadBotModal
        isOpen={isUploadOpen}
        onClose={() => setIsUploadOpen(false)}
        onSuccess={refreshData}
        arenas={arenas}
        teams={teams}
        defaultArenaId={selectedArenaId}
      />

      <TriggerMatchModal
        isOpen={isTriggerMatchOpen}
        onClose={() => setIsTriggerMatchOpen(false)}
        onSuccess={(matchId) => {
          refreshData();
          handleSelectMatch(matchId);
        }}
        arenas={arenas}
        agents={agents}
        defaultArenaId={selectedArenaId}
      />

      <LoginPage
        isOpen={isLoginOpen}
        onClose={() => setIsLoginOpen(false)}
        onSuccess={(u) => {
          setUser(u);
          refreshData();
        }}
      />
    </div>
  );
};
