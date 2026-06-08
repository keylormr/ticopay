# Tico Pay 🇨🇷

Pagos entre personas (P2P) en colones costarricenses. Full-stack: **Go + Postgres** (backend) y **React + Vite** (frontend), con tema de la bandera de Costa Rica.

## Arquitectura

```
ticopay/
├── backend/            API en Go (chi + pgx + JWT)
│   ├── cmd/server/     punto de entrada
│   ├── internal/
│   │   ├── api/        router + handlers HTTP
│   │   ├── auth/       JWT + bcrypt
│   │   ├── config/     carga de env vars
│   │   ├── db/         pool pgx + migraciones embebidas
│   │   ├── models/     structs de dominio
│   │   └── seed/       datos demo
│   └── Dockerfile
├── frontend/           SPA React + Vite + TypeScript
└── docker-compose.yml  Postgres local (puerto 5433)
```

## Correr en local

**1. Base de datos** (Docker):

```bash
docker compose up -d
```

**2. Backend** (necesita Go 1.23):

```bash
cd backend
cp .env.example .env        # opcional; ya hay defaults
go run ./cmd/server         # escucha en :8080, migra y siembra demo
```

**3. Frontend**:

```bash
cd frontend
npm install
cp .env.example .env        # VITE_API_URL=http://localhost:8080
npm run dev                 # http://localhost:5174
```

### Cuentas demo (sembradas con `SEED_DEMO=true`)

| Correo              | Contraseña    | Saldo        |
| ------------------- | ------------- | ------------ |
| maria@ticopay.cr    | `password123` | ₡250 000,00  |
| carlos@ticopay.cr   | `password123` | ₡75 000,00   |

## API

| Método | Ruta                  | Auth | Descripción                       |
| ------ | --------------------- | ---- | --------------------------------- |
| GET    | `/health`             | —    | Estado del servicio + DB          |
| POST   | `/api/auth/register`  | —    | Crear cuenta (devuelve tokens)    |
| POST   | `/api/auth/login`     | —    | Iniciar sesión                    |
| POST   | `/api/auth/refresh`   | —    | Renovar tokens                    |
| GET    | `/api/me`             | ✓    | Usuario + saldo                   |
| GET    | `/api/transactions`   | ✓    | Historial de movimientos          |
| POST   | `/api/transactions`   | ✓    | Enviar dinero a otro usuario      |

Los montos se guardan en **céntimos** (enteros) para evitar errores de redondeo.

## Deploy

Vercel (frontend) + Render (backend Docker) + Neon (Postgres). Ver [DEPLOY.md](DEPLOY.md).
