# Agentrix AI Starter Kit

Kit oficial de entrenamiento y participación en **Agentrix Platform**.
Diseñado para que cualquier participante pueda entrenar, evaluar y exportar agentes de Inteligencia Artificial para el torneo **sin necesidad de compilar código en Rust ni clonar el repositorio completo**.

---

## 📦 Contenido del Kit

```
agentrix-starter-kit/
├── bin/
│   └── agentrix-arbiter          # Motor de simulación Rust oficial (Linux x86_64, 60 ticks/s)
├── sdk/                          # SDK Python agentrix-training (Gymnasium & PettingZoo)
│   ├── agentrix_training/
│   └── pyproject.toml
├── my_bot/                       # Plantilla de bot lista para modificar o entrenar
│   ├── agent.py                  # Código del bot
│   └── agentrix.json             # Manifiesto de ejecución para la plataforma
├── notebooks/
│   └── agentrix_colab_starter.ipynb # Notebook interactivo para Google Colab
├── requirements.txt              # Dependencias mínimas (numpy, gymnasium, pettingzoo)
├── Makefile                      # Comandos rápidos de preparación y entrenamiento
└── README.md                     # Esta guía
```

---

## 🚀 Inicio Rápido (3 Pasos)

### 1. Instalar el entorno local (Linux x86_64 / WSL2)

```bash
# Opción rápida con Makefile:
make setup

# O manualmente:
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
pip install -e sdk/
chmod +x bin/agentrix-arbiter
```

*(Si utilizas Windows nativo o macOS, consulta la sección **Google Colab / WSL2** más abajo).*

---

### 2. Evaluar y Entrenar tu Agente

Mide el desempeño de agentes contra los **5 rivales de referencia oficiales** (Hunter, Zone Controller, Mob Farmer, Survivor, Random):

```bash
agentrix-eval --policies hunter,mob_farmer,survivor,zone_controller,random --seeds 10
```

Entrena una política neuronal con aprendizaje por imitación y búsqueda de políticas sobre datos reales del motor de simulación:

```bash
agentrix-train --matches 15 --epochs 40 --output-dir my_bot
```

Esto generará automáticamente los pesos optimizados en `my_bot/weights.json` y actualizará `my_bot/agent.py`.

---

### 3. Validar y Empaquetar para la Plataforma

Valida que tu bot responda correctamente a los 3 ticks de admisión del torneo y genera el archivo ZIP listo para subir a la plataforma:

```bash
agentrix-pack --bot-dir my_bot --output my_bot.zip
```

Una vez generado `my_bot.zip`, dirígete a la plataforma web de Agentrix y haz clic en **"Subir Bot"**.

---

## ☁️ Uso en Google Colab / Windows / macOS

El binario `bin/agentrix-arbiter` está compilado para **Linux x86_64** (el mismo entorno de producción del torneo).

- **Google Colab (Recomendado para Windows / macOS):**
  Abre el notebook `notebooks/agentrix_colab_starter.ipynb` en Google Colab. Podrás entrenar tu bot con recursos cloud gratuitos y descargar tu `my_bot.zip` directamente a tu equipo con un solo clic.
- **Windows Subsystem for Linux (WSL2):**
  Si estás en Windows, puedes abrir tu terminal de Ubuntu en WSL2 y seguir los pasos normales de Linux.

---

## 🎯 Contrato del Bot

Tu bot se ejecuta como un proceso aislado que se comunica mediante `stdin` / `stdout`:
1. **Fase INIT:** El árbitro envía `{"phase": "INIT"}`. Tu bot debe inicializar sus pesos y responder `{"status": "READY"}\n`.
2. **Fase TICK:** Por cada tick de la partida (60 por segundo), el árbitro envía el estado de la arena (`self`, `units`, `mobs`, `zone`, `tick`). Tu bot debe responder en menos de **50 ms** con:
   ```json
   {"tick": 1, "angle": 1.57, "shoot": true}
   ```
3. **Fase TERMINATE:** Fin de la partida.

¡Buena suerte en la arena!
