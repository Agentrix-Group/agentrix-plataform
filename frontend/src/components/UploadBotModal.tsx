import React, { useState } from 'react';
import { X, UploadCloud, CheckCircle2, AlertCircle, FileArchive, Loader2 } from 'lucide-react';
import { api } from '../api/client';
import { Arena, Team } from '../types';

interface UploadBotModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  arenas: Arena[];
  teams: Team[];
  defaultArenaId: number;
}

export const UploadBotModal: React.FC<UploadBotModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  arenas,
  teams,
  defaultArenaId,
}) => {
  const [file, setFile] = useState<File | null>(null);
  const [botName, setBotName] = useState('');
  const [teamId, setTeamId] = useState<number>(teams[0]?.id || 1);
  const [arenaId, setArenaId] = useState<number>(defaultArenaId);
  const [runtime, setRuntime] = useState('python-standard');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const selected = e.target.files[0];
      setFile(selected);
      if (!botName) {
        setBotName(selected.name.replace(/\.[^/.]+$/, ''));
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) {
      setError('Please select a bot archive (.zip)');
      return;
    }

    setLoading(true);
    setError(null);
    setSuccessMsg(null);

    try {
      const formData = new FormData();
      formData.append('bot_archive', file);
      formData.append('bot_name', botName);
      formData.append('team_id', teamId.toString());
      formData.append('arena_id', arenaId.toString());
      formData.append('runtime', runtime);

      const resp = await api.uploadAgent(formData);
      setSuccessMsg(resp.message || 'Bot uploaded and validated successfully!');
      setTimeout(() => {
        onSuccess();
        onClose();
      }, 1200);
    } catch (err: any) {
      setError(err.message || 'Failed to upload bot');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-lg shadow-xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-800 bg-slate-950/60">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded bg-sky-500/10 border border-sky-500/30 flex items-center justify-center text-sky-400">
              <UploadCloud className="w-4 h-4" />
            </div>
            <h3 className="text-sm font-semibold text-slate-100">Upload Agent / Bot Archive</h3>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-200 p-1 rounded hover:bg-slate-800 transition"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-5 space-y-4 text-xs">
          {error && (
            <div className="p-3 rounded bg-red-950/80 border border-red-800/60 text-red-300 flex items-start gap-2">
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0 text-red-400" />
              <span>{error}</span>
            </div>
          )}

          {successMsg && (
            <div className="p-3 rounded bg-emerald-950/80 border border-emerald-800/60 text-emerald-300 flex items-start gap-2">
              <CheckCircle2 className="w-4 h-4 mt-0.5 shrink-0 text-emerald-400" />
              <span>{successMsg}</span>
            </div>
          )}

          <div>
            <label className="block text-slate-400 font-medium mb-1">Bot Name</label>
            <input
              type="text"
              required
              value={botName}
              onChange={(e) => setBotName(e.target.value)}
              placeholder="e.g. Apex-Hunter-v1"
              className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-2 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-sky-500"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-400 font-medium mb-1">Team</label>
              <select
                value={teamId}
                onChange={(e) => setTeamId(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded px-2.5 py-2 text-slate-200 focus:outline-none focus:border-sky-500"
              >
                {teams.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-slate-400 font-medium mb-1">Target Arena</label>
              <select
                value={arenaId}
                onChange={(e) => setArenaId(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded px-2.5 py-2 text-slate-200 focus:outline-none focus:border-sky-500"
              >
                {arenas.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-slate-400 font-medium mb-1">Runtime Environment</label>
            <select
              value={runtime}
              onChange={(e) => setRuntime(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded px-2.5 py-2 text-slate-200 focus:outline-none focus:border-sky-500"
            >
              <option value="python-standard">Python 3.11 (Standard packages + SDK)</option>
              <option value="rust-musl">Rust (Static musl binary)</option>
              <option value="cpp-native">C++ (Linux x86_64 binary)</option>
              <option value="binary">Generic Script / run.sh Executable</option>
            </select>
          </div>

          {/* File Upload Box */}
          <div>
            <label className="block text-slate-400 font-medium mb-1">Bot Package (.zip archive)</label>
            <div className="border-2 border-dashed border-slate-800 hover:border-slate-700 rounded-lg p-4 text-center bg-slate-950/40 cursor-pointer relative">
              <input
                type="file"
                accept=".zip"
                onChange={handleFileChange}
                className="absolute inset-0 opacity-0 cursor-pointer"
              />
              <div className="flex flex-col items-center justify-center gap-1.5 pointer-events-none">
                <FileArchive className="w-8 h-8 text-slate-500" />
                {file ? (
                  <span className="font-mono text-sky-400 text-xs font-semibold">{file.name} ({(file.size / 1024).toFixed(1)} KB)</span>
                ) : (
                  <>
                    <span className="text-slate-300 font-medium">Click or drag & drop .zip here</span>
                    <span className="text-[10px] text-slate-500">Max size 25 MB. Must contain agentrix.json, run.sh or agent.py</span>
                  </>
                )}
              </div>
            </div>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-800">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !file}
              className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white font-medium transition"
            >
              {loading ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Validating & Deploying...</span>
                </>
              ) : (
                <>
                  <UploadCloud className="w-3.5 h-3.5" />
                  <span>Upload & Register</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
