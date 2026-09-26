import React, { useState } from 'react';
import { Activity, ShieldAlert, Eye, Terminal, Clock, RefreshCw } from 'lucide-react';
import { AuditLog } from '../types';

interface AuditLogsPageProps {
  logs: AuditLog[];
  loading: boolean;
  onRefresh: () => void;
}

export const AuditLogsPage: React.FC<AuditLogsPageProps> = ({ logs, loading, onRefresh }) => {
  const [selectedDetails, setSelectedDetails] = useState<Record<string, any> | null>(null);

  return (
    <div className="space-y-4">
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Activity className="w-5 h-5 text-sky-400" />
          <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
            Platform Audit & Security Event Logs
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

      <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-sm">
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
                  <td colSpan={7} className="text-center py-8 text-slate-500">
                    No audit logs recorded yet.
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id}>
                    <td className="text-center font-mono font-bold text-slate-500">
                      #{log.id}
                    </td>
                    <td className="font-mono text-slate-400 text-[11px]">
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td className="font-medium text-slate-200">{log.username}</td>
                    <td>
                      <span className="font-mono text-[10px] uppercase px-1.5 py-0.5 rounded bg-slate-800 text-sky-300 border border-slate-700">
                        {log.action}
                      </span>
                    </td>
                    <td className="font-mono text-slate-400 text-[11px]">
                      {log.target_type}: {log.target_id}
                    </td>
                    <td className="font-mono text-slate-500 text-[10px]">{log.ip_address}</td>
                    <td className="text-right pr-4">
                      {log.details_json && Object.keys(log.details_json).length > 0 && (
                        <button
                          onClick={() => setSelectedDetails(log.details_json)}
                          className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-mono transition"
                        >
                          <Eye className="w-3 h-3" />
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-5 max-w-lg w-full shadow-2xl">
            <h3 className="text-xs font-bold text-slate-100 uppercase tracking-wide mb-3 font-mono">
              Audit Event Payload
            </h3>
            <pre className="bg-slate-950 p-3 rounded border border-slate-800 text-xs font-mono text-sky-300 max-h-80 overflow-y-auto">
              {JSON.stringify(selectedDetails, null, 2)}
            </pre>
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => setSelectedDetails(null)}
                className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs text-slate-200"
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
