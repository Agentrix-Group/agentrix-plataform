import React, { useState } from 'react';
import { Activity, ShieldAlert, Eye, Terminal, Clock, RefreshCw, X, Shield, Lock } from 'lucide-react';
import { AuditLog } from '../types';

interface AuditLogsPageProps {
  logs: AuditLog[];
  loading: boolean;
  onRefresh: () => void;
}

export const AuditLogsPage: React.FC<AuditLogsPageProps> = ({ logs, loading, onRefresh }) => {
  const [selectedDetails, setSelectedDetails] = useState<Record<string, any> | null>(null);

  const getActionBadge = (action: string) => {
    const act = action.toLowerCase();
    if (act.includes('disqualify') || act.includes('freeze') || act.includes('delete')) {
      return (
        <span className="font-mono text-[10px] uppercase font-bold px-2 py-0.5 rounded bg-rose-50 text-rose-700 border border-rose-200">
          {action}
        </span>
      );
    }
    if (act.includes('login') || act.includes('auth')) {
      return (
        <span className="font-mono text-[10px] uppercase font-bold px-2 py-0.5 rounded bg-blue-50 text-blue-700 border border-blue-200">
          {action}
        </span>
      );
    }
    if (act.includes('upload') || act.includes('register') || act.includes('schedule')) {
      return (
        <span className="font-mono text-[10px] uppercase font-bold px-2 py-0.5 rounded bg-emerald-50 text-emerald-700 border border-emerald-200">
          {action}
        </span>
      );
    }
    return (
      <span className="font-mono text-[10px] uppercase font-bold px-2 py-0.5 rounded bg-slate-100 text-slate-700 border border-slate-200">
        {action}
      </span>
    );
  };

  return (
    <div className="space-y-4 animate-fade-in">
      {/* Top Banner Bar */}
      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-indigo-50 border border-indigo-100 flex items-center justify-center text-indigo-600 shadow-xs">
            <Lock className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-base font-black text-slate-900 tracking-tight uppercase">
              Platform Audit & Security Event Logs
            </h1>
            <p className="text-xs text-slate-500 font-medium">
              Tamper-evident administrative trail, match invocations, and authentication telemetry
            </p>
          </div>
        </div>

        <button
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-semibold bg-white hover:bg-slate-50 text-slate-700 border border-slate-300 shadow-xs hover:shadow transition disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 text-slate-500 ${loading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Audit High-Density Table */}
      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-16 text-center">ID</th>
                <th>Timestamp</th>
                <th>Actor</th>
                <th>Action</th>
                <th>Target</th>
                <th>IP Address</th>
                <th className="text-right pr-4">Details</th>
              </tr>
            </thead>
            <tbody>
              {logs.length === 0 ? (
                <tr>
                  <td colSpan={7} className="text-center py-12 text-slate-400 font-medium">
                    No audit logs recorded yet.
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id} className="hover:bg-blue-50/70 transition-colors">
                    <td className="text-center font-mono font-bold text-slate-400 text-xs">
                      #{log.id}
                    </td>
                    <td className="font-mono text-slate-600 text-xs">
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td className="font-bold text-slate-900 text-xs">
                      {log.username}
                    </td>
                    <td>{getActionBadge(log.action)}</td>
                    <td className="font-mono text-slate-600 text-xs">
                      <span className="font-semibold text-slate-800">{log.target_type}</span>: {log.target_id}
                    </td>
                    <td className="font-mono text-slate-500 text-xs">{log.ip_address}</td>
                    <td className="text-right pr-4">
                      {log.details_json && Object.keys(log.details_json).length > 0 && (
                        <button
                          onClick={() => setSelectedDetails(log.details_json)}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-semibold transition"
                        >
                          <Eye className="w-3.5 h-3.5 text-slate-500" />
                          <span>View JSON</span>
                        </button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* JSON Details Modal */}
      {selectedDetails && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 backdrop-blur-sm p-4">
          <div className="bg-white border border-slate-200 rounded-2xl p-5 sm:p-6 max-w-lg w-full shadow-2xl animate-fade-in space-y-4">
            <div className="flex items-center justify-between border-b border-slate-100 pb-3">
              <h3 className="text-sm font-black text-slate-900 uppercase tracking-wide">
                Audit Event Payload
              </h3>
              <button
                onClick={() => setSelectedDetails(null)}
                className="text-slate-400 hover:text-slate-700 p-1 rounded-lg hover:bg-slate-100 transition"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <pre className="bg-slate-950 p-4 rounded-xl border border-slate-800 text-xs font-mono text-emerald-400 max-h-80 overflow-y-auto shadow-inner">
              {JSON.stringify(selectedDetails, null, 2)}
            </pre>
            <div className="flex justify-end">
              <button
                onClick={() => setSelectedDetails(null)}
                className="px-4 py-2 rounded-lg bg-slate-900 hover:bg-slate-800 text-xs font-bold text-white transition shadow-sm"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
