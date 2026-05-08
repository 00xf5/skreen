# Skreen RMM - System Upgrades (Phase 2.5)

This document outlines the recent architectural and functional upgrades made to the Skreen platform, transforming it from a basic remote access tool into a professional-grade Remote Monitoring and Management (RMM) solution.

## 1. Professional Agent Management (Sidebar)
The agent list has been completely refactored for high data density and operational visibility, mirroring industry-standard tools like ScreenConnect.

- **Rich Metadata Extraction**: The agent now natively extracts and reports:
    - **Hostname**: Displays the machine's actual name instead of a UUID.
    - **Active User**: Live tracking of the currently logged-in Windows/Linux user.
    - **OS Detection**: Automatic OS fingerprinting with dynamic iconography.
- **Live Idle Tracking**: Real-time monitoring of user activity using native system calls (`GetLastInputInfo` on Windows). 
    - **Status States**: "Active" (Green), "Idle Xm/h" (Yellow/Warning), and "Offline" (Gray).
- **Global Search**: Filter agents instantly by hostname, username, or ID.

## 2. Advanced Control Toolbar
The remote viewing experience now includes a professional-grade control surface for managing complex environments.

- **Multi-Monitor Support**: 
    - Automatic detection of all connected displays.
    - Seamless switching between monitors on the fly without session restarts.
- **Special Key Injection**: 
    - **CAD (Ctrl-Alt-Del)**: System-level sequence simulation for administrative tasks.
    - **Win Key**: Native Windows key simulation for quick navigation.
- **Block Remote Input**: 
    - A security/maintenance feature that locks the physical mouse and keyboard on the remote machine while the operator is working.
- **Immersive Fullscreen**: A redesigned auto-hiding floating toolbar that keeps all controls accessible during fullscreen sessions.

## 3. Global Infrastructure & Networking
The underlying connectivity layer has been hardened for real-world deployments where the server, dashboard, and agents are on separate global networks.

- **Dynamic ICE/TURN Brokerage**: 
    - The system no longer uses hardcoded IPs for WebRTC.
    - A new `/api/webrtc-config` endpoint on the Go backend handles dynamic distribution of STUN/TURN credentials.
- **Cloud-Ready (Render + Vercel)**:
    - Optimized for split-cloud deployments (e.g., Backend on Render, Controller on Vercel).
    - Support for enterprise-grade TURN relays (Metered.ca, Twilio, Coturn) via server environment variables:
        - `TURN_URL`
        - `TURN_USERNAME`
        - `TURN_PASSWORD`
- **Native Go sysinfo**: A new cross-platform package in the agent that performs platform-specific system calls without requiring CGO, keeping the binary small and portable.

## Technical Summary
| Feature | Implementation | Component |
| :--- | :--- | :--- |
| Idle Detection | `syscall.LazyDLL` (User32.dll) | Agent (Go) |
| Multi-Monitor | `kbinani/screenshot` enumeration | Agent (Go) |
| Video Stream | WebRTC DataChannel (JPEG/UDP) | Agent/Controller |
| Input Blocking | `user32.BlockInput` API | Agent (Go) |
| UI Styling | Vanilla CSS / High-Density Layout | Controller (React) |

---
*These upgrades prepare the system for Phase 4: Deep System Audit & Telemetry.*
