import React, { useEffect, useState } from 'react';
import {
  Terminal,
  Cpu,
  BookOpen,
  Copy,
  Check,
  Package,
  ShieldAlert,
  ArrowRight,
  UploadCloud,
  CheckCircle2,
  Swords,
  Trophy,
  Zap,
  HardDrive,
  Clock,
  Layers,
  FileCode2,
  RefreshCw,
  Download,
  ExternalLink,
  Cloud,
  Laptop,
  CheckCircle
} from 'lucide-react';
import { Arena, ArenaKitManifest } from '../types';
import { api } from '../api/client';

interface TrainingKitPageProps {
  arena: Arena | null;
  onOpenUpload: () => void;
}

export const TrainingKitPage: React.FC<TrainingKitPageProps> = ({ arena, onOpenUpload }) => {
  const [manifest, setManifest] = useState<ArenaKitManifest | null>(null);
  const [loading, setLoading] = useState(false);
  const [copiedIndex, setCopiedIndex] = useState<string | null>(null);

  useEffect(() => {
    if (!arena) return;
    setLoading(true);
    api.getArenaKit(arena.id)
      .then(setManifest)
      .catch((err: any) => {
        console.error('Failed to load arena kit:', err);
      })
      .finally(() => setLoading(false));
  }, [arena?.id]);

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedIndex(id);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const downloadUrl = arena
    ? `/api/v1/arenas/${arena.id}/kit/download`
    : '/api/v1/arenas/1/kit/download';

  const colabUrl =
    manifest?.starter_kit.colab_url ||
    'https://colab.research.google.com/github/Agentrix-Group/agentrix-plataform/blob/main/notebooks/agentrix_colab_starter.ipynb';

  const formattedSize = manifest?.starter_kit.size_bytes
    ? `${(manifest.starter_kit.size_bytes / 1024).toFixed(0)} KB`
    : '~980 KB';

  const steps = [
    {
      number: '01',
      title: 'Descargar y descomprimir el Starter Kit Oficial',
      desc: 'El kit incluye el motor oficial compilado (bin/agentrix-arbiter), el SDK de entrenamiento, plantillas y rivales. No requiere compilar Rust.',
      cmd: `curl -fsSL -O ${window.location.origin}${downloadUrl}\nunzip agentrix-starter-kit-v0.2.0.zip\ncd agentrix-starter-kit-v0.2.0`,
      id: 'step1',
    },
    {
      number: '02',
      title: 'Instalar el SDK de Entrenamiento',
      desc: 'Instala agentrix-training con soporte para entornos Gymnasium y PettingZoo multi-agente.',
      cmd: 'make setup\n# O manualmente:\npython3 -m venv .venv && source .venv/bin/activate\npip install -r requirements.txt && pip install -e sdk/',
      id: 'step2',
    },
    {
      number: '03',
      title: 'Evaluar contra rivales de referencia (5 Estilos)',
      desc: 'Mide el desempeño de tus agentes contra los 4 estilos heurísticos y el bot aleatorio con rotación de asientos.',
      cmd: 'agentrix-eval --policies hunter,mob_farmer,survivor,zone_controller,random --seeds 10',
      id: 'step3',
    },
    {
      number: '04',
      title: 'Entrenar un agente competitivo',
      desc: 'Entrena una política neuronal con aprendizaje por imitación y búsqueda de políticas sobre datos reales del motor.',
      cmd: 'agentrix-train --matches 15 --epochs 40 --output-dir my_bot',
      id: 'step4',
    },
    {
      number: '05',
      title: 'Validar y Empaquetar para la Plataforma',
      desc: 'Verifica los 3 ticks de admisión real, valida los límites de tamaño y genera el archivo ZIP listo para subir.',
      cmd: 'agentrix-pack --bot-dir my_bot --output my_bot.zip',
      id: 'step5',
    },
  ];

  const baselines = [
    {
      name: 'Hunter (Cazador)',
      style: 'Agresivo',
      badgeColor: 'bg-red-100 text-red-700 border-red-200',
      desc: 'Persigue implacablemente a los bots rivales, prioriza objetivos debilitados y orbita en combate cerrado disparando continuamente.',
    },
    {
      name: 'Zone Controller (Estratégico)',
      style: 'Control de Área',
      badgeColor: 'bg-purple-100 text-purple-700 border-purple-200',
      desc: 'Domina el anillo interior de la zona segura, aprovecha cobertura tras muros y remata oportunidades débiles con alta precisión.',
    },
    {
      name: 'Mob Farmer (Economía)',
      style: 'Recursos / XP',
      badgeColor: 'bg-emerald-100 text-emerald-700 border-emerald-200',
      desc: 'Caza neutrales de manera sistemática para subir de nivel rápidamente, aumentar su HP y daño, y huye ante desventaja táctica.',
    },
    {
      name: 'Survivor / Dodger (Evasión)',
      style: 'Longevidad',
      badgeColor: 'bg-amber-100 text-amber-700 border-amber-200',
      desc: 'Esquiva proyectiles de forma perpendicular, evita el fuego cruzado patrullando el perímetro y prioriza sobrevivir hasta el top.',
    },
    {
      name: 'Random (Ruido Base)',
      style: 'Estocástico',
      badgeColor: 'bg-slate-100 text-slate-700 border-slate-200',
      desc: 'Movimiento y disparos aleatorios. Cualquier agente competitivo debe superarlo con un 100% de victorias.',
    },
  ];

  return (
    <div className="space-y-6">
      {/* Primary Starter Kit Hero Banner */}
      <div className="bg-gradient-to-r from-blue-900 via-slate-900 to-indigo-950 border border-blue-800/40 rounded-2xl p-6 sm:p-8 text-white shadow-lg relative overflow-hidden">
        <div className="absolute right-0 top-0 translate-x-12 -translate-y-8 w-80 h-80 bg-blue-500/10 rounded-full blur-3xl pointer-events-none" />

        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6 relative z-10">
          <div className="max-w-2xl space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <span className="px-2.5 py-0.5 rounded-full text-xs font-black tracking-wider uppercase bg-blue-500 text-white shadow-xs">
                Kit Oficial para Participantes
              </span>
              <span className="px-2.5 py-0.5 rounded-full text-xs font-mono font-bold bg-white/10 text-blue-200 border border-white/10">
                v0.2.0 • Linux x86_64
              </span>
              <span className="px-2.5 py-0.5 rounded-full text-xs font-mono font-medium bg-emerald-500/20 text-emerald-300 border border-emerald-500/30">
                {formattedSize}
              </span>
            </div>

            <h1 className="text-2xl sm:text-3xl font-black text-white tracking-tight flex items-center gap-3">
              <Terminal className="w-8 h-8 text-blue-400" />
              Agentrix AI Starter Kit
            </h1>

            <p className="text-sm text-slate-300 leading-relaxed">
              Paquete autónomo con el motor oficial de producción (<code className="text-blue-300 font-mono">agentrix-arbiter</code> a 60 ticks/s precompilado), el SDK de entrenamiento en Python, plantillas listas de bot y 5 rivales de referencia. <strong>No requiere compilar Rust ni clonar el repositorio.</strong>
            </p>

            {manifest?.starter_kit.sha256 && (
              <div className="flex items-center gap-2 pt-1 text-xs text-slate-400 font-mono">
                <span className="text-slate-500">SHA-256:</span>
                <span className="truncate max-w-[280px] sm:max-w-md bg-black/40 px-2 py-0.5 rounded border border-white/5">
                  {manifest.starter_kit.sha256}
                </span>
                <button
                  onClick={() => copyToClipboard(manifest.starter_kit.sha256 || '', 'sha')}
                  className="p-1 hover:text-white transition cursor-pointer"
                  title="Copiar SHA-256"
                >
                  {copiedIndex === 'sha' ? (
                    <Check className="w-3.5 h-3.5 text-emerald-400" />
                  ) : (
                    <Copy className="w-3.5 h-3.5" />
                  )}
                </button>
              </div>
            )}
          </div>

          <div className="flex flex-col sm:flex-row lg:flex-col gap-3 shrink-0">
            <a
              href={downloadUrl}
              download="agentrix-starter-kit-v0.2.0.zip"
              className="flex items-center justify-center gap-2.5 px-5 py-3 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-bold text-sm shadow-md transition transform hover:-translate-y-0.5 active:translate-y-0"
            >
              <Download className="w-4 h-4" />
              Descargar Starter Kit (.zip)
            </a>

            <a
              href={colabUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-2.5 px-5 py-3 rounded-xl bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 border border-amber-500/40 font-bold text-sm transition"
            >
              <Cloud className="w-4 h-4 text-amber-400" />
              Abrir en Google Colab
              <ExternalLink className="w-3.5 h-3.5 ml-0.5 opacity-70" />
            </a>

            <button
              onClick={onOpenUpload}
              className="flex items-center justify-center gap-2.5 px-5 py-2.5 rounded-xl bg-white/10 hover:bg-white/15 text-slate-200 font-semibold text-sm transition cursor-pointer"
            >
              <UploadCloud className="w-4 h-4 text-blue-300" />
              Subir Bot a la Arena
            </button>
          </div>
        </div>
      </div>

      {/* OS Compatibility Callout */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-white border border-slate-200 rounded-xl p-4 shadow-xs flex items-start gap-3">
          <div className="p-2 rounded-lg bg-blue-50 text-blue-600 shrink-0">
            <Laptop className="w-5 h-5" />
          </div>
          <div>
            <h4 className="text-sm font-bold text-slate-900">Linux x86_64</h4>
            <p className="text-xs text-slate-600 mt-0.5 leading-relaxed">
              Compatibilidad 100% nativa directa. Descomprime el kit y entrena localmente en cualquier distribución moderna.
            </p>
          </div>
        </div>

        <div className="bg-white border border-slate-200 rounded-xl p-4 shadow-xs flex items-start gap-3">
          <div className="p-2 rounded-lg bg-amber-50 text-amber-600 shrink-0">
            <Cloud className="w-5 h-5" />
          </div>
          <div>
            <h4 className="text-sm font-bold text-slate-900">Google Colab (Recomendado)</h4>
            <p className="text-xs text-slate-600 mt-0.5 leading-relaxed">
              Ideal para usuarios de <strong>Windows o Mac</strong>. Entrena en la nube sin configurar entornos locales y descarga tu bot en 1 clic.
            </p>
          </div>
        </div>

        <div className="bg-white border border-slate-200 rounded-xl p-4 shadow-xs flex items-start gap-3">
          <div className="p-2 rounded-lg bg-purple-50 text-purple-600 shrink-0">
            <Terminal className="w-5 h-5" />
          </div>
          <div>
            <h4 className="text-sm font-bold text-slate-900">Windows WSL2</h4>
            <p className="text-xs text-slate-600 mt-0.5 leading-relaxed">
              Compatible ejecutando Ubuntu dentro de WSL2. El binario corre con la misma paridad estricta de producción.
            </p>
          </div>
        </div>
      </div>

      {/* Arena Manifest Specifications Card */}
      <div className="bg-white border border-slate-200 rounded-xl p-6 shadow-xs">
        <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-100">
          <div className="flex items-center gap-2.5">
            <Cpu className="w-5 h-5 text-indigo-600" />
            <h2 className="text-lg font-bold text-slate-900">
              Especificaciones de la Arena: {arena?.name || 'Cargando...'}
            </h2>
          </div>
          <span className="text-xs text-slate-500 font-mono">
            {manifest ? `Arena ID: ${manifest.arena_id} • Fase: ${manifest.phase.toUpperCase()}` : 'Obteniendo manifiesto...'}
          </span>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200/80">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-1 flex items-center gap-1">
              <Zap className="w-3.5 h-3.5 text-blue-600" /> Motor
            </div>
            <div className="text-sm font-black font-mono text-slate-900">
              {manifest?.engine.version || 'agentrix-engine-v1'}
            </div>
            <div className="text-[11px] text-slate-500 mt-0.5">60 ticks/s simulación</div>
          </div>

          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200/80">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-1 flex items-center gap-1">
              <Layers className="w-3.5 h-3.5 text-purple-600" /> Features
            </div>
            <div className="text-sm font-black font-mono text-slate-900">
              {manifest?.engine.feature_encoder_version || 'features-v1'}
            </div>
            <div className="text-[11px] text-slate-500 mt-0.5">83 floats normalizados</div>
          </div>

          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200/80">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-1 flex items-center gap-1">
              <Clock className="w-3.5 h-3.5 text-amber-600" /> Tick Timeout
            </div>
            <div className="text-sm font-black font-mono text-slate-900">
              {manifest?.package_limits.tick_timeout_ms || 50} ms
            </div>
            <div className="text-[11px] text-slate-500 mt-0.5">Warmup: 10.0 s</div>
          </div>

          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200/80">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-1 flex items-center gap-1">
              <HardDrive className="w-3.5 h-3.5 text-emerald-600" /> Memoria
            </div>
            <div className="text-sm font-black font-mono text-slate-900">2.0 GiB</div>
            <div className="text-[11px] text-slate-500 mt-0.5">CPU: 1 core cgroup</div>
          </div>

          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200/80">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-1 flex items-center gap-1">
              <Package className="w-3.5 h-3.5 text-cyan-600" /> Paquete ZIP
            </div>
            <div className="text-sm font-black font-mono text-slate-900">100 MiB máx</div>
            <div className="text-[11px] text-slate-500 mt-0.5">Expandido: 512 MiB</div>
          </div>

          <div className="p-3 rounded-lg bg-slate-50 border border-slate-200/80">
            <div className="text-[11px] font-bold text-slate-500 uppercase tracking-wider mb-1 flex items-center gap-1">
              <FileCode2 className="w-3.5 h-3.5 text-slate-600" /> Runtimes
            </div>
            <div className="text-xs font-bold text-slate-900 flex flex-wrap gap-1 mt-1">
              <span className="px-1.5 py-0.5 rounded bg-blue-100 text-blue-700 font-mono text-[10px]">python</span>
              <span className="px-1.5 py-0.5 rounded bg-emerald-100 text-emerald-700 font-mono text-[10px]">onnx</span>
              <span className="px-1.5 py-0.5 rounded bg-slate-200 text-slate-800 font-mono text-[10px]">binary</span>
            </div>
          </div>
        </div>
      </div>

      {/* Step by Step Quickstart */}
      <div className="bg-white border border-slate-200 rounded-xl p-6 shadow-xs">
        <div className="flex items-center gap-2 mb-4">
          <BookOpen className="w-5 h-5 text-blue-600" />
          <h2 className="text-lg font-bold text-slate-900">Guía Rápida de Preparación</h2>
        </div>

        <div className="space-y-4">
          {steps.map((s) => (
            <div key={s.id} className="border border-slate-200 rounded-lg p-4 bg-slate-50/50 hover:bg-slate-50 transition">
              <div className="flex items-start justify-between gap-4 mb-2">
                <div className="flex items-center gap-3">
                  <span className="w-7 h-7 rounded-lg bg-blue-600 text-white font-mono font-bold text-xs flex items-center justify-center shrink-0">
                    {s.number}
                  </span>
                  <div>
                    <h3 className="text-sm font-bold text-slate-900">{s.title}</h3>
                    <p className="text-xs text-slate-600 mt-0.5">{s.desc}</p>
                  </div>
                </div>
              </div>

              <div className="relative mt-2">
                <pre className="bg-slate-900 text-slate-100 font-mono text-xs p-3 rounded-lg overflow-x-auto selection:bg-blue-500 selection:text-white">
                  <code>{s.cmd}</code>
                </pre>
                <button
                  onClick={() => copyToClipboard(s.cmd, s.id)}
                  className="absolute right-2 top-2 p-1.5 rounded-md bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 transition cursor-pointer"
                  title="Copiar comando"
                >
                  {copiedIndex === s.id ? (
                    <Check className="w-3.5 h-3.5 text-emerald-400" />
                  ) : (
                    <Copy className="w-3.5 h-3.5" />
                  )}
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Baseline Rivals Roster */}
      <div className="bg-white border border-slate-200 rounded-xl p-6 shadow-xs">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Swords className="w-5 h-5 text-amber-600" />
            <h2 className="text-lg font-bold text-slate-900">Rivales de Referencia Oficiales</h2>
          </div>
          <span className="text-xs text-slate-500">Incluidos en agentrix_training.baselines</span>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {baselines.map((b) => (
            <div key={b.name} className="p-4 rounded-lg border border-slate-200 bg-slate-50/60 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between gap-2 mb-2">
                  <h3 className="font-bold text-sm text-slate-900">{b.name}</h3>
                  <span className={`text-[10px] font-bold px-2 py-0.5 rounded-full border ${b.badgeColor}`}>
                    {b.style}
                  </span>
                </div>
                <p className="text-xs text-slate-600 leading-relaxed">{b.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Protocol & Parity Guarantee */}
      <div className="bg-slate-900 text-slate-100 rounded-xl p-6 border border-slate-800 shadow-md">
        <div className="flex items-center gap-2.5 mb-3">
          <CheckCircle2 className="w-5 h-5 text-emerald-400" />
          <h3 className="font-bold text-base text-white">Garantía de Paridad 100% y Sandbox Real</h3>
        </div>
        <p className="text-xs text-slate-300 leading-relaxed max-w-3xl mb-4">
          El proceso <code className="text-blue-300 font-mono">agentrix-arbiter train-env</code> expone la física del
          motor Rust compilado directamente a tus bucles de entrenamiento. Ninguna regla o cálculo se simula por aproximación
          en Python: las observaciones recibidas y las consecuencias de tus acciones en entrenamiento local son
          estrictamente idénticas a las del torneo en producción.
        </p>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
          <div className="bg-slate-800/80 p-3 rounded-lg border border-slate-700">
            <span className="text-emerald-400 font-bold block mb-1">Sin ejecución remota</span>
            El servidor solo admite archivos ZIP probados y ejecutados dentro de Bubblewrap/cgroups.
          </div>
          <div className="bg-slate-800/80 p-3 rounded-lg border border-slate-700">
            <span className="text-blue-400 font-bold block mb-1">Admisión determinista</span>
            La herramienta <code className="text-slate-200 font-mono">agentrix-pack</code> valida 3 ticks completos de respuesta local antes de exportar.
          </div>
          <div className="bg-slate-800/80 p-3 rounded-lg border border-slate-700">
            <span className="text-purple-400 font-bold block mb-1">Rotación de asientos</span>
            El evaluador <code className="text-slate-200 font-mono">agentrix-eval</code> alterna las posiciones para eliminar sesgos en el ranking.
          </div>
        </div>
      </div>
    </div>
  );
};
