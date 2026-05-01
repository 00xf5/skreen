# SCON - Secure Command & Operations Network

A **production-grade**, modern Remote Monitoring & Management (RMM) and Remote Access tool built entirely in Go and React. Engineered to provide an enterprise-tier support experience (similar to ScreenConnect or AnyDesk) with zero external dependencies.

---

## 🎯 What is SCON?
SCON is a blazing-fast, agent-based remote administration platform. It uses a centralized **Hub-and-Spoke** WebSocket server to manage thousands of stateless agents, coupled with a seamless WebRTC pipeline for low-latency visual remote control.

### Why it's different:
- **No CGO Required:** The agent uses pure Win32 API calls to perform stealth operations, persistence, clipboard syncing, and process management, making compilation fast and strictly static.
- **WebRTC Native:** No proprietary video codecs. ScreenView uses WebRTC DataChannels carrying lightweight JPEG frames, meaning it works flawlessly in any modern browser without extensions.
- **NAT-Busting Connectivity:** Built-in TURN server compatibility and WebSocket routing means the agent can sit behind corporate firewalls, hotel Wi-Fi, or CGNAT and still connect back instantly.

---

## 🚀 Core Features

### 🖥️ 1. Real-Time Remote Control (ScreenView)
- **WebRTC Peer-to-Peer:** Low latency remote screen viewing.
- **Dynamic Quality Control:** Change between Low (640x360), Balanced (960x540), and High (1280x720) without dropping the connection.
- **Absolute Mouse/Keyboard Control:** Real-time input scaling and key mapping to the remote host.
- **Clipboard Sync:** Bi-directional text clipboard synchronization.

### ⚙️ 2. System Management
- **Process Manager:** Live GUI tracking of remote process memory with one-click remote `taskkill`.
- **File Transfer:** Drag-and-drop file chunking with dynamic UI progress bars, resume states, and cancel capabilities.
- **Remote Terminal:** Direct, interactive shell execution with output streaming.

### 🛡️ 3. "Click & Forget" Installer
- **Level 1 Persistence (User-Mode):** Runs invisibly in the background and sets a registry Run Key for the current user.
- **Level 2 Persistence (System Service):** If run as Administrator, instantly creates a Highest-Privilege `ONSTART` Scheduled Task, allowing it to survive reboots, run as `NT AUTHORITY\SYSTEM`, and control the Windows Login Screen.
- **Stealth Console:** Built with `-H=windowsgui`, meaning absolutely zero black console flashes when run.

### 🌐 4. Secure Onboarding Flow
- **Invite Links:** Technician generates an 8-character, time-limited code (e.g., `ABCD-1234`).
- **Join Portal:** A premium browser interface for end-users to enter codes and securely download their specific payload.

---

## 🏗️ Architecture

```
┌─────────────────────────────────┐
│     The Network Edge (NAT)      │
│  Caddy (Proxy) + Coturn (TURN)  │
└───────────────┬─────────────────┘
                │
┌───────────────▼─────────────────┐
│      SCON Server (Go hub)       │
│ • WebSockets  • Token Auth      │
│ • HTTP API    • Invite Store    │
└──────┬───────────────────┬──────┘
       │                   │
┌──────▼──────┐     ┌──────▼──────┐
│  SCON Agent │     │  Controller │
│  (Go .exe)  │     │  (React UI) │
└─────────────┘     └─────────────┘
```

1. **Controller:** A dark-teal, glassmorphic React Dashboard. Connects via WSS to manage agents and stream remote inputs.
2. **Server:** A lightweight, thread-safe Go router. It never stores video data; it only authenticates and routes SDP offers and commands.
3. **Agent:** The headless payload running on the target machine. Polls via WebSockets and connects out via WebRTC when ScreenView is initiated.

---

## ⚡ Deployment & Build Guide

### 1. The Build Pipeline (PowerShell)
To compile the platform perfectly, use the provided `build.ps1` script in the root directory. 

```powershell
.\build.ps1
```
**What the script does:**
1. Compiles `agent.exe` with `-H=windowsgui` (stealth) and `-s -w` (size reduction).
2. Automatically copies the built `agent.exe` to the `server/` folder so the `join.html` download API is primed.
3. Compiles the `server.exe`.

### 2. Starting the Infrastructure
You have two ways to run SCON:

**Option A: Development Mode (Localhost)**
```bash
# Terminal 1: Run the Server
cd server
.\server.exe

# Terminal 2: Run the UI
cd controller
npm run dev
```

**Option B: Production Mode (Global Routing)**
To host this globally so agents can connect from anywhere, use the included Docker Compose stack.
1. Update `Caddyfile` with your domain.
2. Run the stack to spin up the Reverse Proxy and TURN Server:
```bash
docker-compose up -d
```
3. Run the Go server manually (or wrap it in its own Dockerfile). 

---

## 🔧 Internal Modules & Protocols

### 📁 File Transfer Flow
SCON does not use HTTP for files. It chunks files directly over the WebSocket bus to bypass firewall upload restrictions:
1. Controller sends `MsgFileReq` with file size and chunk count.
2. Agent opens a target `os.File`.
3. Controller streams Base64 chunks via `MsgFileChunk`.
4. Agent writes chunks to disk and replies with `MsgFileAck`.

### 📺 WebRTC Signaling
1. Technician clicks **View Screen**.
2. React app creates an RTCPeerConnection and an SDP **Offer**.
3. Go Server relays the Offer to the Agent.
4. Agent reads the Offer, creates its own RTCPeerConnection, and replies with an **Answer**.
5. Both sides exchange ICE Candidates (checking STUN and TURN) until a peer-to-peer or relay tunnel is established.

### 🔒 Security Model
- **HMAC Signatures:** Every WebSocket packet contains a SHA-256 HMAC hash.
- **Ephemeral Invites:** Download endpoints only unlock when provided with a non-expired code generated securely by an authenticated Admin.
- **Turn Relay:** The provided `turnserver.conf` uses `lt-cred-mech` ensuring unauthorized clients cannot hijack your WebRTC relay bandwidth.

---

## 🛠️ Project Structure
* `agent/` - Headless Go payload.
  * `/internal/clipboard/` - Pure Win32 clipboard sync.
  * `/internal/fs/` - Chunked file assembly logic.
  * `/internal/installer/` - Privilege escalation and persistence logic.
  * `/internal/screenshare/` - DXGI/GDI screen capture & WebRTC logic.
* `controller/` - React frontend (Vite).
* `server/` - Go HTTP and WebSocket Hub.
* `build.ps1` - Production compilation pipeline.
* `docker-compose.yml` - Infrastructure stack (Caddy/Coturn).

---

## ⚠️ Disclaimer
SCON is an extremely powerful administrative tool. The agent executes commands with SYSTEM-level privileges if installed as an Administrator. This software must only be used on networks and machines where you have explicit, documented authorization.
