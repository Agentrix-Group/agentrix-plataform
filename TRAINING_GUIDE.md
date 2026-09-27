# Guía Oficial de Entrenamiento y Desarrollo de Agentes — Agentrix

Esta guía documenta la arquitectura, el SDK de entrenamiento local, la integración con Gymnasium y PettingZoo, las versiones de protocolo, los bots de referencia y el procedimiento para empaquetar y validar agentes antes de someterlos a la plataforma.

---

## 1. Arquitectura y Filosofía

1. **Un Solo Motor (Single-Engine Parity):**
   - Toda la física del juego, reglas, geometría, combate, creeps/mobs y cálculo de puntajes residen en la librería de Rust (`agentrix-arbiter`).
   - El proceso de entrenamiento interactivo utiliza el comando `agentrix-arbiter train-env`, el cual expone la simulación exacta tick a tick vía IPC (`stdin`/`stdout` JSON).
   - **Garantía:** Las observaciones y resultados obtenidos en entrenamiento local son 100% numéricamente idénticos a los del torneo oficial en producción. Ninguna física se duplica o aproxima en Python.

2. **Seguridad y Ejecución Aislada:**
   - La plataforma de producción **no entrena modelos ni compila código no confiable** en el host.
   - El servidor de producción solo admite archivos `.zip` pre-empaquetados que superen una fase de admisión estricta (calentamiento `INIT` -> `READY` y 3 ticks de simulación real en menos de 50 ms por tick).
   - En producción, los bots corren en cgroups aislados (`systemd-run` / Bubblewrap) con cuotas estrictas de CPU y memoria (2 GiB RAM, 1 vCPU, sin acceso a red).

---

## 2. Versiones de Protocolo

Para asegurar compatibilidad a largo plazo y reproducibilidad de las repeticiones, el motor y el SDK exponen identificadores de versión formales:

| Protocolo / Componente | Versión Actual | Propósito |
|---|---|---|
| `ENGINE_VERSION` | `agentrix-engine-v1` | Física, colisiones y simulación de proyectiles a 60 Hz. |
| `RULES_VERSION` | `agentrix-rules-v1` | Configuración de daño, vida, reaparición de mobs y tormenta. |
| `OBSERVATION_VERSION` | `agentrix-obs-v1` | Esquema JSON transmitido a los bots cada tick. |
| `FEATURE_ENCODER_VERSION` | `agentrix-features-v1` | Vector fijo de 83 características normalizadas en float32 con máscaras. |
| `SCORE_VERSION` | `agentrix-score-v1` | Fórmula de desempate y puntuación del torneo (100 puntos máx). |

---

## 3. Límites y Restricciones de Envío

Cualquier submission que supere los límites del sistema será rechazada inmediatamente:

- **Tamaño del ZIP comprimido:** `<= 100 MiB`
- **Tamaño del contenido descomprimido:** `<= 512 MiB`
- **Tamaño de `agentrix.json`:** `<= 16 KiB`
- **Cantidad máxima de archivos:** `<= 1,000 archivos`
- **Tiempo de Calentamiento (`INIT`):** `<= 10,000 ms` (10 segundos)
- **Límite de tiempo por tick (`TICK`):** `<= 50 ms`
- **Memoria RAM disponible:** `2 GiB` (Swap: 0)
- **Runtimes admitidos:**
  - `python-standard`: Python 3.10+ con librerías estándar y `numpy`.
  - `python-onnx`: Python con `onnxruntime` CPU para inferencia neuronal.
  - `binary`: Binario estático compilado en Linux x86_64 (`bot_bin`).

---

## 4. Instalación del Kit de Desarrollo

### Requisitos
- Linux x86_64 (o Google Colab con entorno Linux)
- Rust 1.75+ y Cargo
- Python 3.10+ y `pip`

### Paso 1: Compilar el árbitro local
```bash
git clone https://github.com/Agentrix-Group/agentrix-platform.git
cd agentrix-platform
cargo build --release -p agentrix-arbiter
```
El binario resultante se ubica en `simulation/arbiter/target/release/agentrix-arbiter`.

### Paso 2: Crear entorno virtual e instalar el SDK
```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -e sdk/
```

Para soporte opcional de Gymnasium y PettingZoo:
```bash
pip install -e "sdk/[gym]"
```

---

## 5. Esquema de Observación y Acciones

En cada tick (a 60 ticks por segundo), el bot recibe en `stdin`:
```json
{
  "phase": "TICK",
  "tick": 450,
  "time": 7.5,
  "you": {
    "pos": [600.0, 375.0],
    "facing": 1.57,
    "hp": 95.0,
    "max_hp": 100.0,
    "speed": 90.0,
    "vision": 250.0,
    "damage": 12.0,
    "level": 2,
    "xp": 35.0,
    "xp_next": 52.0,
    "cooldown": 0.0,
    "alive": true
  },
  "visible_enemies": [
    { "id": 2, "pos": [720.0, 390.0], "hp": 40.0, "max_hp": 100.0, "level": 1 }
  ],
  "visible_mobs": [
    { "id": 14, "pos": [580.0, 310.0], "hp": 20.0, "max_hp": 30.0 }
  ],
  "visible_bullets": [
    { "pos": [650.0, 380.0], "color": 1 }
  ],
  "zone": {
    "center": [600.0, 375.0],
    "radius": 520.0
  },
  "walls": [
    { "x": 300.0, "y": 200.0, "w": 40.0, "h": 120.0 }
  ]
}
```

El bot debe responder en `stdout` una única línea antes de 50 ms:
```json
{
  "tick": 450,
  "angle": 1.5707963,
  "shoot": true
}
```

---

## 6. Bots de Referencia Incluidos

El SDK incluye 5 políticas en `agentrix_training.baselines`:

1. **`HunterPolicy` (Agresivo):**
   - Prioriza a los jugadores enemigos.
   - Cierra distancia hasta el rango de fuego (~140 unidades) y orbita tácticamente disparando continuamente.
2. **`MobFarmerPolicy` (Farming de Recursos):**
   - Prioriza mobs neutrales para acumular XP rápidamente y curarse tras cada subida de nivel.
   - Huye tácticamente ante jugadores hostiles si su HP se encuentra por debajo del 70%.
3. **`SurvivorPolicy` (Evasión / Dodger):**
   - Esquiva proyectiles entrantes perpendicularmente a su trayectoria.
   - Mantiene distancia prudente de zonas de fuego cruzado y patrulla la periferia de la zona segura.
4. **`ZoneControllerPolicy` (Control de Área):**
   - Controla el anillo interior de la tormenta.
   - Utiliza muros como cobertura y remata enemigos con HP crítico (< 35%).
5. **`RandomPolicy` (Estocástico):**
   - Emite acciones aleatorias. Todo agente viable debe batirlo al 100%.

---

## 7. Flujo de Trabajo: Entrenar, Evaluar y Empaquetar

### 7.1 Evaluación en Torneo Local
Evalúa 5 bots simultáneos con rotación cíclica de asientos a lo largo de $N$ partidas:
```bash
agentrix-eval --policies hunter,mob_farmer,survivor,zone_controller,random --seeds 10
```
Salida de ejemplo:
```text
+-----------------+---------+------+----------+-----------+-----------+----------+---------------------+
| Policy          | Matches | Wins | Win Rate | Avg Score | Avg Kills | Avg Mobs | 1st/2nd/3rd/4th/5th |
+-----------------+---------+------+----------+-----------+-----------+----------+---------------------+
| zone_controller | 10      | 4    | 40.0%    | 46.5      | 0.30      | 4.20     | 4/3/1/1/1           |
| hunter          | 10      | 3    | 30.0%    | 39.0      | 0.80      | 6.10     | 3/2/3/1/1           |
| survivor        | 10      | 2    | 20.0%    | 34.0      | 0.00      | 0.10     | 2/1/4/2/1           |
| mob_farmer      | 10      | 1    | 10.0%    | 26.5      | 0.10      | 2.50     | 1/2/1/4/2           |
| random          | 10      | 0    |  0.0%    | 15.0      | 0.00      | 0.50     | 0/2/1/2/5           |
+-----------------+---------+------+----------+-----------+-----------+----------+---------------------+
```

Para guardar la repetición de la mejor partida:
```bash
agentrix-eval --policies hunter,mob_farmer,survivor,zone_controller,random --seeds 10 --save-best-replay exhibicion.json
```

### 7.2 Entrenamiento de un Agente Competitivo
El pipeline en `agentrix-train` recopila datos de combate de expertos contra el motor real y entrena una red neuronal ligera mediante Behavioral Cloning y optimización:
```bash
agentrix-train --matches 15 --epochs 40 --output-dir bots/mi_bot
```
Esto genera en `bots/mi_bot/`:
- `agentrix.json`: Manifiesto oficial del bot.
- `agent.py`: Código ejecutable de inferencia (latencia < 0.5 ms por tick).
- `weights.json`: Matrices de pesos neuronales (32x16).

### 7.3 Validación y Empaquetado Automático
Antes de subir el bot, valida los límites de tamaño y ejecuta la admisión oficial de 3 ticks con el árbitro:
```bash
agentrix-pack --bot-dir bots/mi_bot --output bots/mi_bot.zip
```
Salida esperada:
```text
Running admission check: 'python3 agent.py' in /home/user/agentrix-platform/bots/mi_bot...
Admission check passed! Bot responds with READY and adheres to tick action schema.
Creating package: /home/user/agentrix-platform/bots/mi_bot.zip...
Successfully packaged 'ApexTrainedAgent' v1 -> bots/mi_bot.zip (33.2 KiB)
```

---

## 8. Subida y Participación en la Plataforma Web

1. Ingresa a la interfaz web de Agentrix (`http://localhost:3000`).
2. Haz clic en la pestaña **"Entrenar"** para verificar los parámetros actuales de la arena activa.
3. Haz clic en el botón azul **"Subir Bot a la Arena"** o ve a la pestaña **"Bots & Ingestion"**.
4. Selecciona tu archivo `mi_bot.zip` generado por `agentrix-pack`.
5. La plataforma descomprime en sandbox, valida los 3 ticks de admisión y registra tu versión en el Ladder.
6. Tu bot participará en rondas competitivas y podrás ver las repeticiones interactivas 2D en la pestaña **"Matches"**.
