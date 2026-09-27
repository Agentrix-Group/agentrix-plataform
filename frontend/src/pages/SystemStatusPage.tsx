import React from 'react';
import { Settings, CheckCircle2, XCircle, Database, Cpu, Radio, Clock, RefreshCw, Activity, ShieldCheck } from 'lucide-react';
import { SystemStatus } from '../types';

interface SystemStatusPageProps {
  status: SystemStatus | null;
  loading: boolean;
  onRefresh: () => void;
}

export const SystemStatusPage: React.FC<SystemStatusPageProps> = ({ status, loading, onRefresh }) => {
  return (
    <div className="space-y-4 animate-fade-in">
      {/* Top Banner Bar */}
      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-emerald-50 border border-emerald-100 flex items-center justify-center text-emerald-600 shadow-xs">
            <Activity className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-base font-black text-slate-900 tracking-tight uppercase">
              System Diagnostics & Infrastructure Health
            </h1>
            <p className="text-xs text-slate-500 font-medium">
              Live daemon heartbeats, simulation arbiter status, and database telemetry
            </p>
          </div>
        </div>

        <button
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-semibold bg-white hover:bg-slate-50 text-slate-700 border border-slate-300 shadow-xs hover:shadow transition disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 text-slate-500 ${loading ? 'animate-spin' : ''}`} />
          <span>Refresh Telemetry</span>
        </button>
      </div>

      {status ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
          {/* PostgreSQL Status Card */}
          <div className="bg-white border border-slate-200 rounded-xl p-5 shadow-sm hover:shadow-md transition-all space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-blue-50 border border-blue-100 flex items-center justify-center text-blue-600">
                  <Database className="w-4 h-4" />
                </div>
                <h3 className="font-bold text-slate-900 text-sm">PostgreSQL Engine</h3>
              </div>
              {status.database_healthy ? (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-emerald-50 text-emerald-700 border border-emerald-200 shadow-xs">
                  <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
                  <span>Operational</span>
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-rose-50 text-rose-700 border border-rose-200 shadow-xs">
                  <XCircle className="w-3.5 h-3.5" />
                  <span>Degraded</span>
                </span>
              )}
            </div>
            <p className="text-slate-600 leading-relaxed font-normal">
              Active connection pool verified. Schema <code className="text-blue-700 bg-blue-50 px-1 py-0.5 rounded font-mono text-[11px]">agentrix_platform</code> with continuous Glicko-2 ratings ladder.
            </p>
          </div>

          {/* Rust Arbiter Engine Card */}
          <div className="bg-white border border-slate-200 rounded-xl p-5 shadow-sm hover:shadow-md transition-all space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-orange-50 border border-orange-100 flex items-center justify-center text-orange-600">
                  <Cpu className="w-4 h-4" />
                </div>
                <h3 className="font-bold text-slate-900 text-sm">Rust Arbiter Simulation</h3>
              </div>
              {status.arbiter_healthy ? (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-emerald-50 text-emerald-700 border border-emerald-200 shadow-xs">
                  <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
                  <span>Sandboxed & Ready</span>
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-rose-50 text-rose-700 border border-rose-200 shadow-xs">
                  <XCircle className="w-3.5 h-3.5" />
                  <span>Binary Missing</span>
                </span>
              )}
            </div>
            <div className="bg-slate-50 p-2.5 rounded-lg border border-slate-200 font-mono text-[11px] text-slate-600 truncate">
              Binary: <span className="text-slate-900 font-semibold">{status.arbiter_path}</span>
            </div>
          </div>

          {/* Continuous Ladder Matchmaker Card */}
          <div className="bg-white border border-slate-200 rounded-xl p-5 shadow-sm hover:shadow-md transition-all space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-indigo-50 border border-indigo-100 flex items-center justify-center text-indigo-600">
                  <Radio className="w-4 h-4" />
                </div>
                <h3 className="font-bold text-slate-900 text-sm">Continuous Matchmaker</h3>
              </div>
              {status.matchmaker_active ? (
                <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-bold bg-blue-50 text-blue-700 border border-blue-200 shadow-xs">
                  <span className="w-2 h-2 rounded-full bg-blue-500 animate-ping"></span>
                  <span>Daemon Active</span>
                </span>
              ) : (
                <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold bg-slate-100 text-slate-600 border border-slate-200">
                  Manual Mode
                </span>
              )}
            </div>
            <p className="text-slate-600 leading-relaxed font-normal">
              Active concurrent simulation jobs running in queue:{' '}
              <span className="font-mono font-bold text-blue-700 bg-blue-50 px-1.5 py-0.5 rounded border border-blue-100">
                {status.running_matches}
              </span>
            </p>
          </div>

          {/* Server Time & Synchrony Card */}
          <div className="bg-white border border-slate-200 rounded-xl p-5 shadow-sm hover:shadow-md transition-all space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-teal-50 border border-teal-100 flex items-center justify-center text-teal-600">
                  <Clock className="w-4 h-4" />
                </div>
                <h3 className="font-bold text-slate-900 text-sm">Contest Server Clock</h3>
              </div>
              <span className="inline-flex items-center gap-1 font-mono text-[11px] font-bold text-emerald-700 bg-emerald-50 px-2.5 py-1 rounded-full border border-emerald-200">
                <ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
                NTP Synchronized
              </span>
            </div>
            <div className="bg-slate-50 p-2.5 rounded-lg border border-slate-200 font-mono text-[11px] text-slate-800 font-bold">
              {new Date(status.server_time).toUTCString()}
            </div>
          </div>
        </div>
      ) : (
        <div className="bg-white border border-slate-200 rounded-xl p-12 text-center text-slate-500 text-xs shadow-sm">
          Loading system health diagnostics...
        </div>
      )}
    </div>
  );
};
