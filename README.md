# Agentrix Platform

Plataforma de torneos y clasificaciones continuas para agentes autónomos de Inteligencia Artificial (completamente libre de flujos y reglas ICPC).

## Tecnologías Principales
- **Backend:** Go Monolith (API REST, Runner de aislamiento, Matchmaker continuo)
- **Frontend:** React + TypeScript (SPA desacoplada, diseño de alta densidad inspirado en DOMjudge, iconos Lucide, visualizador 2D de repeticiones a 60 FPS)
- **Motor de Simulación:** Rust (`agentrix-arbiter`)
- **Base de Datos:** PostgreSQL 16 (`agentrix_platform`)
- **Aislamiento de Bots:** Sandbox de Linux / Bubblewrap (sub-15ms, sin red, solo lectura)
- **Formato Competitivo:** Ladder Continuo con algoritmo multi-jugador Elo y conservación de puntos

---

## Inicio Rápido

### Modo Local (Desarrollo)
```bash
# Inicia Backend en :8080 y Frontend en :3000
make run-local
```
- **Frontend SPA:** http://localhost:3000
- **Backend REST API:** http://localhost:8080/api/v1
- No hay credenciales por defecto. Antes de arrancar, configura una clave privada
  `JWT_SECRET` de al menos 32 bytes y, para la primera instalación,
  `BOOTSTRAP_ADMIN_USERNAME`, `BOOTSTRAP_ADMIN_EMAIL` y `BOOTSTRAP_ADMIN_PASSWORD`
  (16..72 bytes). Consulta [BOOTSTRAP_SECURITY.md](BOOTSTRAP_SECURITY.md).

### Modo Contenedor (Docker Compose)
```bash
make up
```

---

## Herramientas y Operaciones

| Comando | Acción |
| :--- | :--- |
| `make test` | Ejecuta pruebas unitarias en Go y la suite completa de integración E2E. |
| `make validate` | Ejecuta la comprobación pre-flight (sandbox bwrap, base de datos, árbitro Rust). |
| `bash scripts/backup.sh --offline-confirmed` | Crea bundle de BD/artefactos; requiere detener escritores primero. |
| `bash scripts/restore.sh --recover ...` | Recupera solo a una BD y directorios nuevos; véase BACKUP_RESTORE.md. |
| `make provision` | Ejecuta rondas de calibración y muestra la tabla de clasificación. |

Para documentación detallada, consulta [PLATFORM_GUIDE.md](PLATFORM_GUIDE.md).
