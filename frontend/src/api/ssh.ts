import type { ConnectionProfile } from "./profiles";
import { app, onRuntimeEvent } from "./wails";

export type StartSSHSessionRequest = {
  profile: ConnectionProfile;
  password?: string;
  passphrase?: string;
  columns: number;
  rows: number;
  timeoutSeconds: number;
};

export type SSHSessionInfo = {
  sessionId: string;
  status: string;
};

export type SSHDataEvent = {
  sessionId: string;
  stream: "stdout" | "stderr";
  data: string;
};

export type SSHCloseEvent = {
  sessionId: string;
  message: string;
};

export type HostKeyPrompt = {
  requestId: string;
  host: string;
  address: string;
  keyType: string;
  fingerprint: string;
  knownHosts: string;
  changed: boolean;
};

export type HostKeyDecision = {
  requestId: string;
  accept: boolean;
};

export async function startSSHSession(
  request: StartSSHSessionRequest,
): Promise<SSHSessionInfo> {
  return app().StartSSHSession(request);
}

export async function sendSSHInput(
  sessionId: string,
  data: string,
): Promise<void> {
  return app().SendSSHInput(sessionId, data);
}

export async function keepAliveSSHSession(sessionId: string): Promise<void> {
  return app().KeepAliveSSHSession(sessionId);
}

export async function getSSHWorkingDirectory(
  sessionId: string,
): Promise<string> {
  return app().GetSSHWorkingDirectory(sessionId);
}

export async function resizeSSHSession(
  sessionId: string,
  columns: number,
  rows: number,
): Promise<void> {
  return app().ResizeSSHSession(sessionId, columns, rows);
}

export async function disconnectSSHSession(sessionId: string): Promise<void> {
  return app().DisconnectSSHSession(sessionId);
}

export async function resolveHostKeyPrompt(
  decision: HostKeyDecision,
): Promise<void> {
  return app().ResolveHostKeyPrompt(decision);
}

export function onSSHData(callback: (event: SSHDataEvent) => void): () => void {
  return onRuntimeEvent("ssh:data", callback);
}

export function onSSHError(
  callback: (event: SSHCloseEvent) => void,
): () => void {
  return onRuntimeEvent("ssh:error", callback);
}

export function onSSHClosed(
  callback: (event: SSHCloseEvent) => void,
): () => void {
  return onRuntimeEvent("ssh:closed", callback);
}

export function onHostKeyPrompt(
  callback: (event: HostKeyPrompt) => void,
): () => void {
  return onRuntimeEvent("ssh:hostkey", callback);
}
