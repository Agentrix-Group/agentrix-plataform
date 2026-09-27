import React, { useState } from 'react';
import { X, UploadCloud, CheckCircle2, AlertCircle, FileArchive, Loader2, Sparkles } from 'lucide-react';
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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);

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

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = () => {
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const dropped = e.dataTransfer.files[0];
      setFile(dropped);
      if (!botName) {
        setBotName(dropped.name.replace(/\.[^/.]+$/, ''));
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) {
      setError('Please select a bot package archive (.zip)');
      return;
    }
    if (file.size > 100 * 1024 * 1024) {
      setError('ZIP exceeds the 100 MiB package limit');
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

      const resp = await api.uploadAgent(formData);
      setSuccessMsg(resp.message || 'Bot package uploaded and sandbox-validated successfully!');
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 backdrop-blur-sm p-4">
      <div className="bg-white border border-slate-200 rounded-2xl shadow-2xl w-full max-w-md overflow-hidden animate-fade-in">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-5 border-b border-slate-100 bg-slate-50/70">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-blue-50 border border-blue-100 flex items-center justify-center text-blue-600 shadow-xs">
              <UploadCloud className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-black text-slate-900 tracking-tight">Upload Bot Package</h3>
              <p className="text-[11px] text-slate-400 font-medium">Algorithmic agent sandbox submission</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-700 p-1.5 rounded-lg hover:bg-slate-100 transition"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4 text-xs">
          {error && (
            <div className="p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-rose-700 flex items-start gap-2.5 font-medium">
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0 text-rose-500" />
              <span>{error}</span>
            </div>
          )}

          {successMsg && (
            <div className="p-3.5 rounded-xl bg-emerald-50 border border-emerald-200 text-emerald-800 flex items-start gap-2.5 font-medium">
              <CheckCircle2 className="w-4 h-4 mt-0.5 shrink-0 text-emerald-600" />
              <span>{successMsg}</span>
            </div>
          )}

          <div>
            <label className="block text-slate-700 font-bold mb-1.5">Agent / Bot Name</label>
            <input
              type="text"
              required
              value={botName}
              onChange={(e) => setBotName(e.target.value)}
              placeholder="e.g. Apollo-Hunter-v1"
              className="w-full bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-slate-900 placeholder-slate-400 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-700 font-bold mb-1.5">Affiliated Team</label>
              <select
                value={teamId}
                onChange={(e) => setTeamId(Number(e.target.value))}
                className="w-full bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-slate-900 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
              >
                {teams.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-slate-700 font-bold mb-1.5">Target Contest Arena</label>
              <select
                value={arenaId}
                onChange={(e) => setArenaId(Number(e.target.value))}
                className="w-full bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-slate-900 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
              >
                {arenas.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* File Upload Box */}
          <div>
            <label className="block text-slate-700 font-bold mb-1.5">Bot Package (.zip archive)</label>
            <div
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              className={`border-2 border-dashed rounded-xl p-5 text-center cursor-pointer relative transition-all ${
                isDragging
                  ? 'border-blue-500 bg-blue-50/80 scale-[1.01]'
                  : 'border-slate-300 hover:border-blue-400 bg-slate-50/60'
              }`}
            >
              <input
                type="file"
                accept=".zip"
                onChange={handleFileChange}
                className="absolute inset-0 opacity-0 cursor-pointer"
              />
              <div className="flex flex-col items-center justify-center gap-2 pointer-events-none">
                <div className="w-10 h-10 rounded-full bg-blue-100/70 text-blue-600 flex items-center justify-center">
                  <FileArchive className="w-5 h-5" />
                </div>
                {file ? (
                  <div className="space-y-0.5">
                    <span className="font-mono text-blue-700 text-xs font-bold block">{file.name}</span>
                    <span className="text-[11px] text-slate-500 font-medium">
                      {(file.size / 1024).toFixed(1)} KB — Ready to upload
                    </span>
                  </div>
                ) : (
                  <>
                    <span className="text-slate-800 font-bold text-xs">Click or drag & drop ZIP archive here</span>
                    <span className="text-[10px] text-slate-400">
                      Limit 100 MiB. Must contain root <code className="bg-slate-200 px-1 py-0.2 rounded font-mono text-slate-700">agentrix.json</code> manifest.
                    </span>
                  </>
                )}
              </div>
            </div>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-2.5 pt-3 border-t border-slate-100">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 font-semibold transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !file}
              className="inline-flex items-center gap-1.5 px-5 py-2 rounded-lg bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-white font-bold shadow-xs hover:shadow active:scale-[0.98] disabled:opacity-50 transition"
            >
              {loading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Validating & Deploying...</span>
                </>
              ) : (
                <>
                  <UploadCloud className="w-4 h-4" />
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
