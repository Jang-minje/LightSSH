import type { StartSSHSessionRequest } from "./ssh";
import { app, onRuntimeEvent } from "./wails";

export type SFTPConnectRequest = {
  ssh: StartSSHSessionRequest;
};

export type SFTPSessionInfo = {
  sessionId: string;
  status: string;
};

export type RemoteEntry = {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  mode: string;
  modTime: string;
};

export type LocalEntry = {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  modTime: string;
};

export type TransferRequest = {
  sessionId: string;
  localPath: string;
  remotePath: string;
  overwrite: boolean;
};

export type TerminalTransferRequest = {
  sshSessionId: string;
  localPath: string;
  remotePath: string;
  overwrite: boolean;
};

export type TransferInfo = {
  transferId: string;
  status: string;
};

export type TransferProgressEvent = {
  transferId: string;
  sessionId: string;
  direction: "upload" | "download";
  localPath: string;
  remotePath: string;
  bytes: number;
  total: number;
  status: "started" | "in_progress" | "completed" | "failed" | "canceled";
  message?: string;
};

export async function startSFTPSession(
  request: SFTPConnectRequest,
): Promise<SFTPSessionInfo> {
  return app().StartSFTPSession(request);
}

export async function closeSFTPSession(sessionId: string): Promise<void> {
  return app().CloseSFTPSession(sessionId);
}

export async function listRemoteDirectory(
  sessionId: string,
  remotePath: string,
): Promise<RemoteEntry[]> {
  return app().ListRemoteDirectory(sessionId, remotePath);
}

export async function listLocalDirectory(
  localPath: string,
): Promise<LocalEntry[]> {
  return app().ListLocalDirectory(localPath);
}

export async function defaultLocalDirectory(): Promise<string> {
  return app().DefaultLocalDirectory();
}

export async function selectLocalDirectory(current: string): Promise<string> {
  return app().SelectLocalDirectory(current);
}

export async function openLocalDirectory(localPath: string): Promise<void> {
  return app().OpenLocalDirectory(localPath);
}

export async function revealLocalPath(localPath: string): Promise<void> {
  return app().RevealLocalPath(localPath);
}

export async function uploadFile(request: TransferRequest): Promise<TransferInfo> {
  return app().UploadFile(request);
}

export async function downloadFile(
  request: TransferRequest,
): Promise<TransferInfo> {
  return app().DownloadFile(request);
}

export async function getSSHRemoteUser(sessionId: string): Promise<string> {
  return app().GetSSHRemoteUser(sessionId);
}

export async function listRemoteDirectoryViaTerminal(
  sessionId: string,
  remotePath: string,
): Promise<RemoteEntry[]> {
  return app().ListRemoteDirectoryViaTerminal(sessionId, remotePath);
}

export async function uploadFileViaTerminal(
  request: TerminalTransferRequest,
): Promise<TransferInfo> {
  return app().UploadFileViaTerminal(request);
}

export async function downloadFileViaTerminal(
  request: TerminalTransferRequest,
): Promise<TransferInfo> {
  return app().DownloadFileViaTerminal(request);
}

export async function cancelFileTransfer(transferId: string): Promise<void> {
  return app().CancelFileTransfer(transferId);
}

export async function makeRemoteDirectory(
  sessionId: string,
  remotePath: string,
): Promise<void> {
  return app().MakeRemoteDirectory(sessionId, remotePath);
}

export async function renameRemotePath(
  sessionId: string,
  oldPath: string,
  newPath: string,
): Promise<void> {
  return app().RenameRemotePath(sessionId, oldPath, newPath);
}

export async function deleteRemotePath(
  sessionId: string,
  remotePath: string,
): Promise<void> {
  return app().DeleteRemotePath(sessionId, remotePath);
}

export function onSFTPProgress(
  callback: (event: TransferProgressEvent) => void,
): () => void {
  return onRuntimeEvent("sftp:progress", callback);
}
