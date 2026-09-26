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
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-emerald-950/80 text-emerald-400 border border-emerald-800/60 ${className}`}>
        <CheckCircle2 className="w-3 h-3 text-emerald-400" />
        <span>Active</span>
      </span>
    );
  }

  if (norm === 'finished') {
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-sky-950/80 text-sky-400 border border-sky-800/60 ${className}`}>
        <CheckCircle2 className="w-3 h-3 text-sky-400" />
        <span>Finished</span>
      </span>
    );
  }

  if (norm === 'running') {
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-amber-950/80 text-amber-400 border border-amber-800/60 animate-pulse ${className}`}>
        <Play className="w-3 h-3 text-amber-400" />
        <span>Running</span>
      </span>
    );
  }

  if (norm === 'scheduled' || norm === 'pending_validation') {
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-slate-800 text-slate-300 border border-slate-700 ${className}`}>
        <Clock className="w-3 h-3 text-slate-400" />
        <span>Scheduled</span>
      </span>
    );
  }

  if (norm === 'disqualified') {
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-rose-950/80 text-rose-400 border border-rose-800/60 ${className}`}>
        <ShieldAlert className="w-3 h-3 text-rose-400" />
        <span>Disqualified</span>
      </span>
    );
  }

  if (norm === 'failed' || norm === 'error' || norm === 'degraded') {
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-red-950/80 text-red-400 border border-red-800/60 ${className}`}>
        <XCircle className="w-3 h-3 text-red-400" />
        <span>Failed</span>
      </span>
    );
  }

  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-slate-800 text-slate-300 border border-slate-700 ${className}`}>
      <AlertTriangle className="w-3 h-3 text-slate-400" />
      <span>{status}</span>
    </span>
  );
};
