import type { StartSSHSessionRequest } from "./ssh";
import { app, onRuntimeEvent, optionalApp } from "./wails";

export type LocalTunnelConfig = {
  id: string;
  name: string;
  localBindAddress: string;
  localPort: number;
  targetHost: string;
  targetPort: number;
};

export type ReverseTunnelConfig = {
  id: string;
  name: string;
  remoteBindAddress: string;
  remotePort: number;
  localTargetHost: string;
  localTargetPort: number;
};

export type StartLocalTunnelRequest = {
  ssh: StartSSHSessionRequest;
  config: LocalTunnelConfig;
};

export type StartReverseTunnelRequest = {
  ssh: StartSSHSessionRequest;
  config: ReverseTunnelConfig;
};

export type TunnelInfo = {
  tunnelId: string;
  direction: "local" | "reverse";
  name: string;
  localAddress: string;
  localPort: number;
  remoteAddress: string;
  remotePort: number;
  targetHost: string;
  targetPort: number;
  status: string;
  connectionCount: number;
  message?: string;
};

export type TunnelEvent = {
  tunnelId: string;
  status: string;
  connectionCount: number;
  message?: string;
};

export async function startLocalTunnel(
  request: StartLocalTunnelRequest,
): Promise<TunnelInfo> {
  return app().StartLocalTunnel(request);
}

export async function stopLocalTunnel(tunnelId: string): Promise<void> {
  return app().StopLocalTunnel(tunnelId);
}

export async function startReverseTunnel(
  request: StartReverseTunnelRequest,
): Promise<TunnelInfo> {
  return app().StartReverseTunnel(request);
}

export async function listTunnels(): Promise<TunnelInfo[]> {
  return optionalApp()?.ListTunnels() ?? [];
}

export function onTunnelEvent(callback: (event: TunnelEvent) => void): () => void {
  return onRuntimeEvent("tunnel:event", callback);
}
