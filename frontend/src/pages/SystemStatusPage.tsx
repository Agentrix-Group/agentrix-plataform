import React from 'react';
import { Settings, CheckCircle2, XCircle, Database, Cpu, Radio, Clock, RefreshCw } from 'lucide-react';
import { SystemStatus } from '../types';

interface SystemStatusPageProps {
  status: SystemStatus | null;
  loading: boolean;
  onRefresh: () => void;
}

export const SystemStatusPage: React.FC<SystemStatusPageProps> = ({ status, loading, onRefresh }) => {
  return (
    <div className="space-y-4">
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Settings className="w-5 h-5 text-sky-400" />
          <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
            System Diagnostics & Infrastructure Health
          </h1>
        </div>

        <button
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {status ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
          {/* PostgreSQL Status Card */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-5 shadow-sm space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Database className="w-4 h-4 text-sky-400" />
                <h3 className="font-bold text-slate-100">PostgreSQL Database Engine</h3>
              </div>
              {status.database_healthy ? (
                <span className="flex items-center gap-1 text-emerald-400 font-medium">
                  <CheckCircle2 className="w-3.5 h-3.5" />
                  <span>Connected</span>
                </span>
              ) : (
                <span className="flex items-center gap-1 text-rose-400 font-medium">
                  <XCircle className="w-3.5 h-3.5" />
                  <span>Degraded</span>
                </span>
              )}
            </div>
            <p className="text-slate-400">
              Connection pool active. Schema `agentrix_platform` verified with multi-agent continuous ladder tables.
            </p>
          </div>

          {/* Rust Arbiter Engine Card */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-5 shadow-sm space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Cpu className="w-4 h-4 text-sky-400" />
                <h3 className="font-bold text-slate-100">Rust Arbiter Simulation Engine</h3>
              </div>
              {status.arbiter_healthy ? (
                <span className="flex items-center gap-1 text-emerald-400 font-medium">
                  <CheckCircle2 className="w-3.5 h-3.5" />
                  <span>Ready</span>
                </span>
              ) : (
                <span className="flex items-center gap-1 text-rose-400 font-medium">
                  <XCircle className="w-3.5 h-3.5" />
                  <span>Binary Missing</span>
                </span>
              )}
            </div>
            <div className="bg-slate-950 p-2 rounded border border-slate-800/80 font-mono text-[11px] text-slate-400 truncate">
              {status.arbiter_path}
            </div>
          </div>

          {/* Continuous Ladder Matchmaker Card */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-5 shadow-sm space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Radio className="w-4 h-4 text-sky-400" />
                <h3 className="font-bold text-slate-100">Continuous Matchmaker Daemon</h3>
              </div>
              {status.matchmaker_active ? (
                <span className="flex items-center gap-1 text-emerald-400 font-medium">
                  <CheckCircle2 className="w-3.5 h-3.5" />
                  <span>Running</span>
                </span>
              ) : (
                <span className="text-slate-500 font-medium">Manual Mode</span>
              )}
            </div>
            <p className="text-slate-400">
              Active concurrent match executions in queue: <span className="font-mono font-bold text-slate-200">{status.running_matches}</span>
            </p>
          </div>

          {/* Server Time & Synchrony Card */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-5 shadow-sm space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Clock className="w-4 h-4 text-sky-400" />
                <h3 className="font-bold text-slate-100">Server Clock & Telemetry</h3>
              </div>
              <span className="font-mono text-emerald-400 font-medium">NTP Synchronized</span>
            </div>
            <div className="bg-slate-950 p-2 rounded border border-slate-800/80 font-mono text-[11px] text-slate-300">
              {new Date(status.server_time).toUTCString()}
            </div>
          </div>
        </div>
      ) : (
        <div className="p-8 text-center text-slate-500 text-xs">
          Loading system health status...
        </div>
      )}
    </div>
  );
};
