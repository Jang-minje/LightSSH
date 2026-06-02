import type { ConnectionProfile } from "./profiles";
import type { AppearanceSettings } from "./settings";
import type {
  SSHCloseEvent,
  SSHDataEvent,
  SSHSessionInfo,
  StartSSHSessionRequest,
  HostKeyDecision,
} from "./ssh";
import type {
  LocalEntry,
  RemoteEntry,
  SFTPConnectRequest,
  SFTPSessionInfo,
  TransferInfo,
  TransferProgressEvent,
  TransferRequest,
  TerminalTransferRequest,
} from "./sftp";
import type {
  StartLocalTunnelRequest,
  StartReverseTunnelRequest,
  TunnelEvent,
  TunnelInfo,
} from "./tunnel";

export function app(): WailsApp {
  const service = window.go?.app?.App;
  if (!service) {
    throw new Error("Wails backend is not available.");
  }
  return service;
}

export function optionalApp(): WailsApp | undefined {
  return window.go?.app?.App;
}

export function onRuntimeEvent<T>(
  eventName: string,
  callback: (event: T) => void,
): () => void {
  return window.runtime?.EventsOn(eventName, callback) ?? (() => undefined);
}

export function onNativeFileDrop(
  callback: (x: number, y: number, paths: string[]) => void,
  useDropTarget = true,
): () => void {
  window.runtime?.OnFileDrop(callback, useDropTarget);
  return () => window.runtime?.OnFileDropOff();
}

export async function readClipboardText(): Promise<string> {
  if (window.runtime?.ClipboardGetText) {
    try {
      return await window.runtime.ClipboardGetText();
    } catch {
      // Fall through to browser clipboard API. Wails clipboard can fail
      // transiently when the WebView focus changes around context menus.
    }
  }
  if (navigator.clipboard?.readText) {
    try {
      return await navigator.clipboard.readText();
    } catch {
      return "";
    }
  }
  return "";
}

export async function writeClipboardText(text: string): Promise<boolean> {
  if (window.runtime?.ClipboardSetText) {
    try {
      if (await window.runtime.ClipboardSetText(text)) {
        return true;
      }
    } catch {
      // Fall through to browser/legacy copy below.
    }
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall through to legacy copy for focused WebView cases.
    }
  }
  return legacyCopyText(text);
}

function legacyCopyText(text: string): boolean {
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    return document.execCommand("copy");
  } finally {
    document.body.removeChild(textarea);
  }
}

export type WailsApp = {
  LoadProfiles(): Promise<ConnectionProfile[]>;
  SaveProfiles(profiles: ConnectionProfile[]): Promise<void>;
  SaveProfile(
    profile: ConnectionProfile,
    password: string,
    savePassword: boolean,
  ): Promise<ConnectionProfile>;
  DeleteProfile(profileId: string): Promise<void>;
  DefaultLocalDirectory(): Promise<string>;
  SelectLocalDirectory(current: string): Promise<string>;
  OpenLocalDirectory(localPath: string): Promise<void>;
  RevealLocalPath(localPath: string): Promise<void>;
  LoadAppearanceSettings(): Promise<AppearanceSettings>;
  SaveAppearanceSettings(settings: AppearanceSettings): Promise<void>;
  StartSSHSession(request: StartSSHSessionRequest): Promise<SSHSessionInfo>;
  SendSSHInput(sessionId: string, data: string): Promise<void>;
  KeepAliveSSHSession(sessionId: string): Promise<void>;
  GetSSHWorkingDirectory(sessionId: string): Promise<string>;
  ResizeSSHSession(
    sessionId: string,
    columns: number,
    rows: number,
  ): Promise<void>;
  DisconnectSSHSession(sessionId: string): Promise<void>;
  ResolveHostKeyPrompt(decision: HostKeyDecision): Promise<void>;
  StartSFTPSession(request: SFTPConnectRequest): Promise<SFTPSessionInfo>;
  CloseSFTPSession(sessionId: string): Promise<void>;
  ListRemoteDirectory(
    sessionId: string,
    remotePath: string,
  ): Promise<RemoteEntry[]>;
  ListLocalDirectory(localPath: string): Promise<LocalEntry[]>;
  UploadFile(request: TransferRequest): Promise<TransferInfo>;
  DownloadFile(request: TransferRequest): Promise<TransferInfo>;
  GetSSHRemoteUser(sessionId: string): Promise<string>;
  ListRemoteDirectoryViaTerminal(
    sessionId: string,
    remotePath: string,
  ): Promise<RemoteEntry[]>;
  UploadFileViaTerminal(request: TerminalTransferRequest): Promise<TransferInfo>;
  DownloadFileViaTerminal(
    request: TerminalTransferRequest,
  ): Promise<TransferInfo>;
  CancelFileTransfer(transferId: string): Promise<void>;
  MakeRemoteDirectory(sessionId: string, remotePath: string): Promise<void>;
  RenameRemotePath(
    sessionId: string,
    oldPath: string,
    newPath: string,
  ): Promise<void>;
  DeleteRemotePath(sessionId: string, remotePath: string): Promise<void>;
  StartLocalTunnel(request: StartLocalTunnelRequest): Promise<TunnelInfo>;
  StartReverseTunnel(request: StartReverseTunnelRequest): Promise<TunnelInfo>;
  StopLocalTunnel(tunnelId: string): Promise<void>;
  ListTunnels(): Promise<TunnelInfo[]>;
};

type WailsRuntime = {
  ClipboardGetText(): Promise<string>;
  ClipboardSetText(text: string): Promise<boolean>;
  EventsOn<T>(eventName: string, callback: (event: T) => void): () => void;
  OnFileDrop(
    callback: (x: number, y: number, paths: string[]) => void,
    useDropTarget: boolean,
  ): void;
  OnFileDropOff(): void;
};

declare global {
  interface Window {
    go?: {
      app?: {
        App?: WailsApp;
      };
    };
    runtime?: WailsRuntime;
  }
}

export type WailsEvent =
  | SSHDataEvent
  | SSHCloseEvent
  | TransferProgressEvent
  | TunnelEvent;
