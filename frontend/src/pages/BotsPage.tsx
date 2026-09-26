import React, { useState } from 'react';
import { Bot, UploadCloud, ShieldAlert, CheckCircle2, RefreshCw, FileCode, Hash, Terminal } from 'lucide-react';
import { AgentVersion, Arena, User } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

interface BotsPageProps {
  agents: AgentVersion[];
  arenas: Arena[];
  user: User | null;
  loading: boolean;
  onRefresh: () => void;
  onOpenUpload: () => void;
}

export const BotsPage: React.FC<BotsPageProps> = ({
  agents,
  arenas,
  user,
  loading,
  onRefresh,
  onOpenUpload,
}) => {
  const [actionLoadingId, setActionLoadingId] = useState<number | null>(null);

  const handleDisqualify = async (agentId: number) => {
    const reason = prompt('Enter reason for disqualification:', 'Protocol violation or runtime crash');
    if (!reason) return;

    setActionLoadingId(agentId);
    try {
      await api.disqualifyAgent(agentId, reason);
      onRefresh();
    } catch (err: any) {
      alert(err.message || 'Error disqualifying agent');
    } finally {
      setActionLoadingId(null);
    }
  };

  const handleEnable = async (agentId: number) => {
    setActionLoadingId(agentId);
    try {
      await api.enableAgent(agentId);
      onRefresh();
    } catch (err: any) {
      alert(err.message || 'Error enabling agent');
    } finally {
      setActionLoadingId(null);
    }
  };

  return (
    <div className="space-y-4">
      {/* Top Banner Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot className="w-5 h-5 text-sky-400" />
          <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
            Autonomous Agents & Bot Ingestion
          </h1>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
          <button
            onClick={onOpenUpload}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-semibold bg-sky-600 hover:bg-sky-500 text-white shadow-sm transition"
          >
            <UploadCloud className="w-3.5 h-3.5" />
            <span>Upload New Bot</span>
          </button>
        </div>
      </div>

      {/* Bots High-Density Table */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-16 text-center">ID</th>
                <th>Bot Name</th>
                <th>Team</th>
                <th>Runtime</th>
                <th>Entrypoint</th>
                <th>SHA-256 Checksum</th>
                <th className="text-center">Status</th>
                <th className="text-right pr-4">Actions</th>
              </tr>
            </thead>
            <tbody>
              {agents.length === 0 ? (
                <tr>
                  <td colSpan={8} className="text-center py-8 text-slate-500">
                    No agents uploaded yet.
                  </td>
                </tr>
              ) : (
                agents.map((agent) => (
                  <tr key={agent.id}>
                    <td className="text-center font-mono font-bold text-slate-400">
                      #{agent.id}
                    </td>
                    <td className="font-semibold text-slate-100 flex items-center gap-1.5">
                      <Bot className="w-3.5 h-3.5 text-sky-400" />
                      <span>{agent.name}</span>
                      <span className="text-[10px] text-slate-500 font-mono">v{agent.version}</span>
                    </td>
                    <td className="text-slate-300">{agent.team_name || 'Team'}</td>
                    <td className="font-mono text-slate-400 text-[11px] uppercase">
                      {agent.runtime}
                    </td>
                    <td className="font-mono text-slate-400 text-[11px]">
                      {agent.entrypoint}
                    </td>
                    <td className="font-mono text-slate-500 text-[10px]">
                      {agent.sha256 ? `${agent.sha256.substring(0, 12)}...` : '-'}
                    </td>
                    <td className="text-center">
                      <StatusBadge status={agent.status} />
                    </td>
                    <td className="text-right pr-4">
                      {user?.role === 'admin' && (
                        <div className="flex items-center justify-end gap-2">
                          {agent.status === 'active' ? (
                            <button
                              onClick={() => handleDisqualify(agent.id)}
                              disabled={actionLoadingId === agent.id}
                              className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-rose-950/80 hover:bg-rose-900 text-rose-300 border border-rose-800 transition"
                            >
                              <ShieldAlert className="w-3 h-3 text-rose-400" />
                              <span>Disqualify</span>
                            </button>
                          ) : (
                            <button
                              onClick={() => handleEnable(agent.id)}
                              disabled={actionLoadingId === agent.id}
                              className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-emerald-950/80 hover:bg-emerald-900 text-emerald-300 border border-emerald-800 transition"
                            >
                              <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                              <span>Re-enable</span>
                            </button>
                          )}
                        </div>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
