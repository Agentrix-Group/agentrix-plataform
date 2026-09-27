import React from 'react';
import { CheckCircle2, AlertTriangle, Play, Clock, XCircle, ShieldAlert } from 'lucide-react';

interface StatusBadgeProps {
  status: string;
  className?: string;
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status, className = '' }) => {
  const norm = status?.toLowerCase() || '';

  if (norm === 'active' || norm === 'operational' || norm === 'healthy') {
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-300 shadow-xs ${className}`}>
        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
        <CheckCircle2 className="w-3 h-3 text-emerald-600" />
        <span>Active</span>
      </span>
    );
  }

  if (norm === 'finished') {
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-blue-50 text-blue-700 border border-blue-300 shadow-xs ${className}`}>
        <CheckCircle2 className="w-3 h-3 text-blue-600" />
        <span>Finished</span>
      </span>
    );
  }

  if (norm === 'running') {
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-bold bg-amber-50 text-amber-800 border border-amber-300 shadow-xs ring-2 ring-amber-400/30 ${className}`}>
        <span className="relative flex h-2 w-2">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
          <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
        </span>
        <Play className="w-3 h-3 text-amber-600 fill-amber-600" />
        <span>Running</span>
      </span>
    );
  }

  if (norm === 'scheduled' || norm === 'pending_validation') {
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium bg-slate-100 text-slate-700 border border-slate-300 shadow-xs ${className}`}>
        <Clock className="w-3 h-3 text-slate-500" />
        <span>Scheduled</span>
      </span>
    );
  }

  if (norm === 'disqualified') {
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-bold bg-rose-50 text-rose-700 border border-rose-300 shadow-xs ${className}`}>
        <ShieldAlert className="w-3 h-3 text-rose-600" />
        <span>Disqualified</span>
      </span>
    );
  }

  if (norm === 'failed' || norm === 'error' || norm === 'degraded') {
    return (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-bold bg-red-50 text-red-700 border border-red-300 shadow-xs ${className}`}>
        <XCircle className="w-3 h-3 text-red-600" />
        <span>Failed</span>
      </span>
    );
  }

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium bg-slate-100 text-slate-700 border border-slate-300 shadow-xs ${className}`}>
      <AlertTriangle className="w-3 h-3 text-slate-500" />
      <span>{status}</span>
    </span>
  );
};
