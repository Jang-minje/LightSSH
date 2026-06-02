# Project Instructions

## Product
This project is a lightweight internal SSH maintenance client.

## Tech Stack
- Backend: Go
- Desktop framework: Wails
- Frontend: TypeScript + React or Svelte
- Terminal UI: xterm.js
- SSH: golang.org/x/crypto/ssh
- SFTP: github.com/pkg/sftp/v2

## Core Features
- SSH terminal sessions
- Saved connection profiles
- Password, private key, and ssh-agent authentication
- SFTP upload/download with drag-and-drop
- Local port forwarding
- Remote/reverse port forwarding
- Connection logs and tunnel status display

## Non-Goals
- No Telnet
- No FTP
- No Serial
- No RDP
- No X11 forwarding in MVP
- No raw crypto implementation
- No PuTTY source-code copying

## Security Rules
- Never store plaintext passwords.
- Require host key verification.
- Store only host, port, username, key path, and profile metadata by default.
- Use OS credential storage only when a dedicated credential module is implemented.
- Never log passwords, private keys, passphrases, or session secrets.
- Show explicit warnings for unknown or changed host keys.

## Coding Rules
- Split features into small modules.
- Keep SSH, SFTP, tunnel, config, and UI layers separate.
- Add unit tests for config parsing, profile validation, tunnel config validation, and path handling.
- Add integration-test stubs where real SSH servers are required.
- Before finishing a task, run formatting, linting, and tests where available.