# Skreen Deployment Guide 🚀

This repository is organized as a monorepo containing the **Controller** (React), **Server** (Go), and **Agent** (Go).

## 1. Repository Structure
```text
/                    (Root)
  /controller        -> Deploy to Vercel
  /server            -> Deploy to Render
  /agent             -> Built locally, served by Server
  /installer         -> NSIS scripts
  build.ps1          -> Local build script
```

## 2. Deployment: Backend (Render)
1. **Target**: Go 1.22+ Service.
2. **Root Directory**: `server`
3. **Build Command**: `go build -o server ./cmd`
4. **Start Command**: `./server`
5. **Environment Variables**:
   - `PORT`: Automatically set by Render.
   - `CONTROLLER_URL`: The URL of your Vercel deployment (e.g., `https://skreen-admin.vercel.app`).
   - `SCON_SECRET`: A long random string for securing your management API.

> [!IMPORTANT]
> Because Render runs on Linux, it cannot build the Windows `.exe` installer. You must **commit `server/skreen-agent-setup.exe`** to your repository so the server can find it and serve it to users.

## 3. Deployment: Frontend (Vercel)
1. **Target**: Vite (React).
2. **Root Directory**: `controller`
3. **Build Command**: `npm run build`
4. **Output Directory**: `dist`
5. **Environment Variables**:
   - `VITE_API_URL`: The URL of your Render backend (e.g., `https://skreen-api.onrender.com`).

## 4. How the Flow Works
1. **Technician** logs into the Vercel dashboard.
2. **Technician** clicks "+ New Session" -> Gets a `join_url` (Vercel URL).
3. **End User** opens the Vercel link -> Validates against Render backend.
4. **End User** downloads the agent -> Backend serves the pre-built `skreen-agent-setup.exe`.
5. **Agent** installs and connects to the Render backend via WebSocket.
6. **Technician** sees the agent appear in real-time in the Vercel dashboard.

## 5. Pre-Deployment Check
Run `.\build.ps1` locally to ensure the latest `skreen-agent-setup.exe` is generated and staged in the `server/` folder before you push to GitHub.
