import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type WheelEvent,
} from "react";
import { ConnectionProfileCard } from "../components/ConnectionProfileCard";
import { HelpModal } from "../components/HelpModal";
import { HostKeyModal } from "../components/HostKeyModal";
import { RemoteDownloadModal } from "../components/RemoteDownloadModal";
import { SettingsModal } from "../components/SettingsModal";
import { TerminalPanel } from "../components/TerminalPanel";
import { TunnelPanel } from "../components/TunnelPanel";
import { UploadConfirmModal } from "../components/UploadConfirmModal";
import {
  deleteProfile,
  loadProfiles,
  saveProfile,
  type ConnectionProfile,
} from "../api/profiles";
import {
  closeSFTPSession,
  defaultLocalDirectory,
  downloadFile,
  downloadFileViaTerminal,
  getSSHRemoteUser,
  listRemoteDirectory,
  listRemoteDirectoryViaTerminal,
  onSFTPProgress,
  openLocalDirectory,
  revealLocalPath,
  selectLocalDirectory,
  startSFTPSession,
  uploadFile,
  uploadFileViaTerminal,
  type RemoteEntry,
  type TransferProgressEvent,
} from "../api/sftp";
import {
  disconnectSSHSession,
  getSSHWorkingDirectory,
  keepAliveSSHSession,
  onHostKeyPrompt,
  onSSHClosed,
  onSSHData,
  onSSHError,
  resolveHostKeyPrompt,
  resizeSSHSession,
  sendSSHInput,
  startSSHSession,
  type HostKeyPrompt,
} from "../api/ssh";
import {
  defaultAppearanceSettings,
  loadAppearanceSettings,
  saveAppearanceSettings,
  type AppearanceSettings,
} from "../api/settings";
import { onTunnelEvent } from "../api/tunnel";
import {
  appendAppLog,
  readClipboardText,
  revealAppLogFile,
  writeClipboardText,
} from "../api/wails";

const emptyProfile: ConnectionProfile = {
  id: "manual",
  name: "수동 접속",
  host: "",
  port: 22,
  username: "",
  authType: "password",
};

type TerminalTab = {
  id: string;
  sessionId: string | null;
  title: string;
  profile: ConnectionProfile;
  password: string;
  passphrase: string;
  status: "connecting" | "connected" | "closed" | "error";
  message: string;
  columns: number;
  rows: number;
};

type TerminalMenuState = {
  selection: string;
  tabId: string;
  x: number;
  y: number;
};

type RemoteDownloadState = {
  entries: RemoteEntry[];
  loading: boolean;
  localDirectory: string;
  mode: "sftp" | "terminal";
  overwrite: boolean;
  remotePath: string;
  selectedPaths: Set<string>;
  sessionId: string;
  tabId: string;
};

type DownloadOpenBatch = {
  completed: number;
  directory: string;
  files: string[];
  remaining: Set<string>;
};

type TransferMetrics = {
  averageBytesPerSecond: number;
  lastAt: number;
  lastBytes: number;
  speedBytesPerSecond: number;
  startedAt: number;
};

type ActiveView = "terminal" | "tunnel" | "log";

type AppLogEntry = {
  id: string;
  at: string;
  level: "info" | "warn" | "error";
  source: "ssh" | "sftp" | "tunnel" | "terminal" | "clipboard" | "app";
  message: string;
};

export function HomePage() {
  const [profiles, setProfiles] = useState<ConnectionProfile[]>([]);
  const [profile, setProfile] = useState<ConnectionProfile>(emptyProfile);
  const [password, setPassword] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [passwordSaved, setPasswordSaved] = useState(false);
  const [sessionKeepAlive, setSessionKeepAlive] = useState(false);
  const [terminalTabs, setTerminalTabs] = useState<TerminalTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [terminalFocusKey, setTerminalFocusKey] = useState(0);
  const [leftPanelCollapsed, setLeftPanelCollapsed] = useState(false);
  const [leftPanelWidth, setLeftPanelWidth] = useState(280);
  const [rightPanelCollapsed, setRightPanelCollapsed] = useState(false);
  const [rightPanelWidth, setRightPanelWidth] = useState(220);
  const [activeView, setActiveView] = useState<ActiveView>("terminal");
  const [status, setStatus] = useState("연결 안 됨");
  const [error, setError] = useState("");
  const [hostKeyPrompt, setHostKeyPrompt] = useState<HostKeyPrompt | null>(null);
  const [helpOpen, setHelpOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [appearance, setAppearance] = useState<AppearanceSettings>(
    defaultAppearanceSettings,
  );
  const [remoteUploadPath, setRemoteUploadPath] = useState("/");
  const [uploadPath, setUploadPath] = useState<string | null>(null);
  const [uploadTransferMode, setUploadTransferMode] = useState<
    "sftp" | "terminal"
  >("sftp");
  const [terminalMenu, setTerminalMenu] = useState<TerminalMenuState | null>(
    null,
  );
  const [remoteDownload, setRemoteDownload] =
    useState<RemoteDownloadState | null>(null);
  const [transfers, setTransfers] = useState<Record<string, TransferProgressEvent>>(
    {},
  );
  const [transferMetrics, setTransferMetrics] = useState<
    Record<string, TransferMetrics>
  >({});
  const [logs, setLogs] = useState<AppLogEntry[]>([]);

  const terminalTabsRef = useRef<TerminalTab[]>([]);
  const writerRefs = useRef(new Map<string, (data: string) => void>());
  const pendingOutputRefs = useRef(new Map<string, string[]>());
  const sessionToTabRef = useRef(new Map<string, string>());
  const appearanceLoadedRef = useRef(false);
  const downloadTransferBatchRef = useRef(new Map<string, string>());
  const downloadOpenBatchRef = useRef(new Map<string, DownloadOpenBatch>());

  const activeTab = useMemo(
    () => terminalTabs.find((tab) => tab.id === activeTabId) ?? null,
    [activeTabId, terminalTabs],
  );

  const appendLog = useCallback(
    (
      level: AppLogEntry["level"],
      source: AppLogEntry["source"],
      message: string,
    ) => {
      const entry = {
        id: crypto.randomUUID(),
        at: formatLogTimestamp(new Date()),
        level,
        source,
        message: sanitizeLogMessage(message),
      };
      appendAppLog(formatLogEntry(entry)).catch(() => undefined);
      setLogs((current) => {
        const next = [...current, entry];
        return next.slice(-500);
      });
    },
    [],
  );

  const copyLogs = useCallback(async () => {
    try {
      await writeClipboardText(formatLogEntries(logs));
      setStatus("로그를 클립보드에 복사했습니다.");
    } catch (err: unknown) {
      setError(errorMessage(err));
    }
  }, [logs]);

  const revealLogs = useCallback(async () => {
    try {
      await revealAppLogFile();
      setStatus("로그 파일 위치를 열었습니다.");
    } catch (err: unknown) {
      setError(errorMessage(err));
    }
  }, []);

  useEffect(() => {
    terminalTabsRef.current = terminalTabs;
  }, [terminalTabs]);

  useEffect(() => {
    loadAppearanceSettings()
      .then((settings) => {
        appearanceLoadedRef.current = true;
        setAppearance(settings);
      })
      .catch((err: unknown) => setError(errorMessage(err)));
  }, []);

  useEffect(() => {
    if (!appearanceLoadedRef.current) {
      return;
    }
    saveAppearanceSettings(appearance).catch((err: unknown) =>
      setError(errorMessage(err)),
    );
  }, [appearance]);

  useEffect(() => {
    const blockBrowserZoom = (event: globalThis.WheelEvent) => {
      if (event.ctrlKey) {
        event.preventDefault();
      }
    };
    const options: AddEventListenerOptions = { capture: true, passive: false };
    window.addEventListener("wheel", blockBrowserZoom, options);
    return () => window.removeEventListener("wheel", blockBrowserZoom, options);
  }, []);

  useEffect(() => {
    loadProfiles()
      .then((loadedProfiles) => {
        setProfiles(loadedProfiles);
        if (loadedProfiles[0]) {
          selectProfile(loadedProfiles[0]);
        }
      })
      .catch((err: unknown) => setError(errorMessage(err)));
  }, []);

  useEffect(() => {
    return onHostKeyPrompt((prompt) => {
      appendLog(
        "warn",
        "ssh",
        `host key confirmation required for ${prompt.host} (${prompt.address})`,
      );
      setHostKeyPrompt(prompt);
    });
  }, [appendLog]);

  useEffect(() => {
    return onSFTPProgress((event) => {
      if (event.status !== "in_progress") {
        appendLog(
          event.status === "failed" ? "error" : "info",
          "sftp",
          `${event.direction} ${event.status}, transferId=${event.transferId}, mode=${
            event.transferId.startsWith("terminal:") ? "terminal" : "sftp"
          }, bytes=${event.bytes}/${event.total}`,
        );
      }
      const now = Date.now();
      setTransferMetrics((current) => {
        const previous = current[event.transferId];
        const startedAt = previous?.startedAt ?? now;
        const secondsSinceLast = Math.max(
          0.001,
          (now - (previous?.lastAt ?? startedAt)) / 1000,
        );
        const byteDelta = Math.max(
          0,
          event.bytes - (previous?.lastBytes ?? 0),
        );
        const elapsed = Math.max(0.001, (now - startedAt) / 1000);
        return {
          ...current,
          [event.transferId]: {
            averageBytesPerSecond: event.bytes / elapsed,
            lastAt: now,
            lastBytes: event.bytes,
            speedBytesPerSecond:
              event.status === "in_progress" ? byteDelta / secondsSinceLast : 0,
            startedAt,
          },
        };
      });
      setTransfers((current) => ({ ...current, [event.transferId]: event }));
      if (
        event.sessionId !== remoteDownload?.sessionId &&
        (event.status === "completed" ||
          event.status === "failed" ||
          event.status === "canceled")
      ) {
        closeSFTPSession(event.sessionId).catch(() => undefined);
      }
      if (event.status === "failed" && event.message) {
        setError(event.message);
      }
      if (event.direction === "download" && isFinishedTransfer(event.status)) {
        const batchId = downloadTransferBatchRef.current.get(event.transferId);
        const batch = batchId
          ? downloadOpenBatchRef.current.get(batchId)
          : undefined;
        if (batchId && batch) {
          downloadTransferBatchRef.current.delete(event.transferId);
          batch.remaining.delete(event.transferId);
          if (event.status === "completed") {
            batch.completed += 1;
            batch.files.push(event.localPath);
          }
          if (batch.remaining.size === 0) {
            downloadOpenBatchRef.current.delete(batchId);
            if (batch.completed > 0) {
              const opener =
                batch.files.length === 1
                  ? revealLocalPath(batch.files[0])
                  : openLocalDirectory(batch.directory);
              opener.catch((err: unknown) => setError(errorMessage(err)));
            }
          }
        }
      }
      if (isFinishedTransfer(event.status)) {
        window.setTimeout(() => {
          setTransfers((current) => {
            if (current[event.transferId]?.status !== event.status) {
              return current;
            }
            const next = { ...current };
            delete next[event.transferId];
            return next;
          });
          setTransferMetrics((current) => {
            const next = { ...current };
            delete next[event.transferId];
            return next;
          });
        }, 8000);
      }
    });
  }, [appendLog, remoteDownload?.sessionId]);

  const writeToTab = useCallback((tabId: string, data: string) => {
    const writer = writerRefs.current.get(tabId);
    if (writer) {
      writer(data);
      return;
    }
    const pending = pendingOutputRefs.current.get(tabId) ?? [];
    pending.push(data);
    pendingOutputRefs.current.set(tabId, pending);
  }, []);

  useEffect(() => {
    const offData = onSSHData((event) => {
      const tabId = sessionToTabRef.current.get(event.sessionId);
      if (tabId) {
        writeToTab(tabId, event.data);
      }
    });
    const offError = onSSHError((event) => {
      appendLog("error", "ssh", `session error, sessionId=${event.sessionId}: ${event.message}`);
      const tabId = sessionToTabRef.current.get(event.sessionId);
      if (tabId) {
        setTerminalTabs((current) =>
          current.map((tab) =>
            tab.id === tabId
              ? { ...tab, status: "error", message: event.message }
              : tab,
          ),
        );
      }
      setError(event.message);
    });
    const offClosed = onSSHClosed((event) => {
      appendLog("info", "ssh", `session closed, sessionId=${event.sessionId}: ${event.message}`);
      const tabId = sessionToTabRef.current.get(event.sessionId);
      if (tabId) {
        sessionToTabRef.current.delete(event.sessionId);
        setTerminalTabs((current) =>
          current.map((tab) =>
            tab.id === tabId
              ? {
                  ...tab,
                  sessionId: null,
                  status: "closed",
                  message: event.message,
                }
              : tab,
          ),
        );
      }
      setStatus(event.message);
    });

    return () => {
      offData();
      offError();
      offClosed();
    };
  }, [appendLog, writeToTab]);

  useEffect(() => {
    return onTunnelEvent((event) => {
      appendLog(
        event.status === "error" ? "error" : "info",
        "tunnel",
        `tunnel ${event.status}, id=${event.tunnelId}, connections=${event.connectionCount}${
          event.message ? `, message=${event.message}` : ""
        }`,
      );
    });
  }, [appendLog]);

  useEffect(() => {
    if (!sessionKeepAlive) {
      return;
    }
    const timer = window.setInterval(() => {
      for (const tab of terminalTabsRef.current) {
        if (tab.sessionId && tab.status === "connected") {
          keepAliveSSHSession(tab.sessionId).catch((err: unknown) =>
            {
              const message = errorMessage(err);
              appendLog(
                "error",
                "ssh",
                `keepalive failed, sessionId=${tab.sessionId}: ${message}`,
              );
              setError(message);
            },
          );
        }
      }
    }, 60000);
    return () => window.clearInterval(timer);
  }, [appendLog, sessionKeepAlive]);

  useEffect(() => {
    if (!terminalMenu) {
      return;
    }
    const close = (event: PointerEvent) => {
      const target = event.target;
      if (
        target instanceof Element &&
        target.closest(".terminal-context-menu")
      ) {
        return;
      }
      setTerminalMenu(null);
    };
    const closeByEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setTerminalMenu(null);
      }
    };
    window.addEventListener("pointerdown", close);
    window.addEventListener("keydown", closeByEscape);
    return () => {
      window.removeEventListener("pointerdown", close);
      window.removeEventListener("keydown", closeByEscape);
    };
  }, [terminalMenu]);

  const registerTabWriter = useCallback(
    (tabId: string, writer: (data: string) => void) => {
      writerRefs.current.set(tabId, writer);
      const pending = pendingOutputRefs.current.get(tabId);
      if (pending) {
        pending.forEach((data) => writer(data));
        pendingOutputRefs.current.delete(tabId);
      }
    },
    [],
  );

  const selectProfile = useCallback((savedProfile: ConnectionProfile) => {
    setProfile(savedProfile);
    setPassword("");
    setPassphrase("");
    setPasswordSaved(Boolean(savedProfile.passwordSaved));
    setStatus(`${savedProfile.name} 프로필 선택됨`);
    appendLog(
      "info",
      "app",
      `profile selected, name=${savedProfile.name}, host=${savedProfile.host}:${savedProfile.port}, user=${savedProfile.username}`,
    );
  }, [appendLog]);

  const activateTerminalTab = useCallback((tab: TerminalTab) => {
    setActiveTabId(tab.id);
    setActiveView("terminal");
    setTerminalFocusKey((current) => current + 1);
    setProfile(tab.profile);
    setPassword(tab.password);
    setPassphrase(tab.passphrase);
    setPasswordSaved(Boolean(tab.profile.passwordSaved));
    setStatus(`${tab.title} 터미널 선택됨`);
    appendLog(
      "info",
      "terminal",
      `tab activated, title=${tab.title}, sessionId=${tab.sessionId ?? "none"}, status=${tab.status}`,
    );
  }, [appendLog]);

  const updateProfileDraft = useCallback(
    (changes: Partial<ConnectionProfile>) => {
      const cloningSavedProfile = profile.id !== "manual";
      setProfile((current) => ({
        ...current,
        ...changes,
        id: current.id !== "manual" ? "manual" : current.id,
        passwordSaved: false,
      }));
      if (cloningSavedProfile) {
        setPassword("");
        setPasswordSaved(false);
        setStatus("프로필 편집 중");
      }
    },
    [profile.id],
  );

  const connect = useCallback(async () => {
    setError("");
    setStatus("연결 중...");
    const tabId = crypto.randomUUID();
    const tabProfile = { ...profile };
    const title = tabTitle(tabProfile);
    appendLog(
      "info",
      "ssh",
      `connect requested, title=${title}, host=${tabProfile.host}:${tabProfile.port}, user=${tabProfile.username}, auth=${tabProfile.authType}`,
    );
    try {
      const session = await startSSHSession({
        profile: tabProfile,
        password,
        passphrase,
        columns: 80,
        rows: 24,
        timeoutSeconds: 15,
      });
      sessionToTabRef.current.set(session.sessionId, tabId);
      setTerminalTabs((current) => [
        ...current,
        {
          id: tabId,
          sessionId: session.sessionId,
          title,
          profile: tabProfile,
          password,
          passphrase,
          status: "connected",
          message: "연결됨",
          columns: 80,
          rows: 24,
        },
      ]);
      setActiveTabId(tabId);
      setTerminalFocusKey((current) => current + 1);
      setActiveView("terminal");
      setStatus("연결됨");
      writeToTab(tabId, "\r\nSSH 연결이 시작되었습니다.\r\n");
      appendLog(
        "info",
        "ssh",
        `connect succeeded, title=${title}, sessionId=${session.sessionId}`,
      );
    } catch (err: unknown) {
      const message = errorMessage(err);
      if (message.includes("호스트 키 확인")) {
        setStatus("호스트 키 확인 대기");
        setError("");
        appendLog("warn", "ssh", `connect paused for host key confirmation, title=${title}`);
        return;
      }
      setStatus("연결 실패");
      setError(message);
      appendLog("error", "ssh", `connect failed, title=${title}: ${message}`);
    }
  }, [appendLog, passphrase, password, profile, writeToTab]);

  const disconnectActive = useCallback(async () => {
    if (!activeTab?.sessionId) {
      return;
    }
    try {
      appendLog(
        "info",
        "ssh",
        `disconnect requested, title=${activeTab.title}, sessionId=${activeTab.sessionId}`,
      );
      await disconnectSSHSession(activeTab.sessionId);
    } catch (err: unknown) {
      const message = errorMessage(err);
      appendLog("error", "ssh", `disconnect failed: ${message}`);
      setError(message);
    }
  }, [activeTab, appendLog]);

  const closeTab = useCallback(
    async (tab: TerminalTab) => {
      if (tab.sessionId) {
        appendLog(
          "info",
          "terminal",
          `tab close requested, title=${tab.title}, sessionId=${tab.sessionId}`,
        );
        await disconnectSSHSession(tab.sessionId).catch(() => undefined);
        sessionToTabRef.current.delete(tab.sessionId);
      }
      writerRefs.current.delete(tab.id);
      pendingOutputRefs.current.delete(tab.id);
      const nextTabs = terminalTabsRef.current.filter((item) => item.id !== tab.id);
      if (activeTabId === tab.id) {
        const nextActiveTab = nextTabs[nextTabs.length - 1] ?? null;
        setActiveTabId(nextActiveTab?.id ?? null);
        if (nextActiveTab) {
          setProfile(nextActiveTab.profile);
          setPassword(nextActiveTab.password);
          setPassphrase(nextActiveTab.passphrase);
          setPasswordSaved(Boolean(nextActiveTab.profile.passwordSaved));
          setStatus(`${nextActiveTab.title} 터미널 선택됨`);
        }
      }
      setTerminalTabs((current) => {
        const next = current.filter((item) => item.id !== tab.id);
        return next;
      });
    },
    [activeTabId, appendLog],
  );

  const sendInput = useCallback((tab: TerminalTab, data: string) => {
    if (!tab.sessionId) {
      return;
    }
    sendSSHInput(tab.sessionId, data).catch((err: unknown) => {
      const message = errorMessage(err);
      appendLog(
        "error",
        "ssh",
        `send input failed, title=${tab.title}, sessionId=${tab.sessionId}: ${message}`,
      );
      setError(message);
    });
  }, [appendLog]);

  const copyTerminalSelection = useCallback(async (selection: string) => {
    if (!selection.trim()) {
      appendLog("warn", "clipboard", "copy skipped, empty selection");
      return;
    }
    try {
      await writeClipboardText(selection);
      setStatus("클립보드에 복사됨");
      appendLog(
        "info",
        "clipboard",
        `copy succeeded, selectionLength=${selection.length}`,
      );
    } catch (err: unknown) {
      const message = errorMessage(err);
      appendLog(
        "error",
        "clipboard",
        `copy failed, selectionLength=${selection.length}: ${message}`,
      );
      setError(message);
    }
  }, [appendLog]);

  const pasteClipboardToTab = useCallback(async (tab: TerminalTab | null) => {
    if (!tab?.sessionId) {
      appendLog("warn", "clipboard", "paste skipped, no active SSH session");
      return;
    }
    try {
      const text = await readClipboardText();
      if (!text) {
        appendLog("warn", "clipboard", "paste skipped, clipboard is empty");
        return;
      }
      await sendSSHInput(tab.sessionId, normalizeTerminalPaste(text));
      setTerminalFocusKey((current) => current + 1);
      appendLog(
        "info",
        "clipboard",
        `paste sent, title=${tab.title}, textLength=${text.length}`,
      );
    } catch (err: unknown) {
      const message = errorMessage(err);
      appendLog("error", "clipboard", `paste failed: ${message}`);
      setError(message);
    }
  }, [appendLog]);

  const resizeTerminal = useCallback(
    (tabId: string, columns: number, rows: number) => {
      setTerminalTabs((current) =>
        current.map((tab) =>
          tab.id === tabId ? { ...tab, columns, rows } : tab,
        ),
      );
      const tab = terminalTabsRef.current.find((item) => item.id === tabId);
      if (!tab?.sessionId) {
        return;
      }
      resizeSSHSession(tab.sessionId, columns, rows).catch((err: unknown) =>
        setError(errorMessage(err)),
      );
    },
    [],
  );

  const startPanelResize = useCallback(
    (side: "left" | "right", event: ReactPointerEvent<HTMLDivElement>) => {
      event.preventDefault();
      const startX = event.clientX;
      const startWidth = side === "left" ? leftPanelWidth : rightPanelWidth;
      document.body.classList.add("panel-resizing");

      const handleMove = (moveEvent: PointerEvent) => {
        const delta = moveEvent.clientX - startX;
        if (side === "left") {
          setLeftPanelWidth(clampNumber(startWidth + delta, 220, 460));
        } else {
          setRightPanelWidth(clampNumber(startWidth - delta, 180, 420));
        }
      };
      const handleUp = () => {
        document.body.classList.remove("panel-resizing");
        window.removeEventListener("pointermove", handleMove);
        window.removeEventListener("pointerup", handleUp);
        setTerminalFocusKey((current) => current + 1);
      };

      window.addEventListener("pointermove", handleMove);
      window.addEventListener("pointerup", handleUp, { once: true });
    },
    [leftPanelWidth, rightPanelWidth],
  );

  const saveCurrentProfile = useCallback(async () => {
    setError("");
    try {
      const saved = await saveProfile(profile, password, passwordSaved);
      const loadedProfiles = await loadProfiles();
      setProfiles(loadedProfiles);
      if (saved) {
        selectProfile(saved);
      }
      setStatus("프로필 저장됨");
    } catch (err: unknown) {
      setError(errorMessage(err));
    }
  }, [password, passwordSaved, profile, selectProfile]);

  const uploadDroppedFile = useCallback(
    async (localPath: string, overwrite: boolean, targetName?: string) => {
      setError("");
      setUploadPath(null);
      if (!activeTab) {
        setError("업로드할 터미널 탭을 선택하세요.");
        return;
      }
      const remotePath = joinRemotePath(
        remoteUploadPath,
        targetName ?? baseName(localPath),
      );
      appendLog(
        "info",
        "sftp",
        `upload requested, mode=${uploadTransferMode}, local=${baseName(localPath)}, remote=${remotePath}, overwrite=${overwrite}`,
      );
      try {
        if (uploadTransferMode === "terminal") {
          if (!activeTab.sessionId) {
            setError("현재 터미널 계정 업로드는 연결된 터미널이 필요합니다.");
            return;
          }
          await uploadFileViaTerminal({
            sshSessionId: activeTab.sessionId,
            localPath,
            remotePath,
            overwrite,
          });
          setStatus("현재 터미널 계정 기준으로 업로드 중");
          appendLog(
            "info",
            "sftp",
            `terminal upload started, title=${activeTab.title}, remote=${remotePath}`,
          );
          setActiveView("terminal");
          return;
        }
        const session = await startSFTPSession({
          ssh: {
            profile: activeTab.profile,
            password: activeTab.password,
            passphrase: activeTab.passphrase,
            columns: 80,
            rows: 24,
            timeoutSeconds: 15,
          },
        });
        await uploadFile({
          sessionId: session.sessionId,
          localPath,
          remotePath,
          overwrite,
        });
        appendLog(
          "info",
          "sftp",
          `sftp upload started, sessionId=${session.sessionId}, remote=${remotePath}`,
        );
        setActiveView("terminal");
      } catch (err: unknown) {
        appendLog("error", "sftp", `upload failed before fallback: ${errorMessage(err)}`);
        if (activeTab.sessionId) {
          const retryWithTerminal = window.confirm(
            `SFTP 업로드에 실패했습니다.\n\n${errorMessage(err)}\n\n현재 터미널 계정 방식으로 재시도할까요?\n이 방식은 느릴 수 있습니다.`,
          );
          if (!retryWithTerminal) {
            setError(errorMessage(err));
            return;
          }
          try {
            await uploadFileViaTerminal({
              sshSessionId: activeTab.sessionId,
              localPath,
              remotePath,
              overwrite,
            });
            setStatus("SFTP 실패로 터미널 계정 기준 업로드 중");
            appendLog(
              "warn",
              "sftp",
              `fallback terminal upload started, title=${activeTab.title}, remote=${remotePath}`,
            );
            setActiveView("terminal");
            return;
          } catch (fallbackErr: unknown) {
            const message = errorMessage(fallbackErr);
            appendLog("error", "sftp", `fallback terminal upload failed: ${message}`);
            setError(message);
            return;
          }
        }
        setError(errorMessage(err));
      }
    },
    [activeTab, appendLog, remoteUploadPath, uploadTransferMode],
  );

  const handleTerminalFileDrop = useCallback(
    async (tab: TerminalTab, path: string) => {
      setActiveView("terminal");
      setActiveTabId(tab.id);
      setError("");
      appendLog(
        "info",
        "terminal",
        `file dropped on terminal, title=${tab.title}, file=${baseName(path)}`,
      );
      if (!tab.sessionId) {
        setError("SSH 연결 후 터미널에 파일을 드래그하세요.");
        return;
      }
      try {
        const workingDirectory = await getSSHWorkingDirectory(tab.sessionId);
        setRemoteUploadPath(workingDirectory || "/");
        setUploadTransferMode("sftp");
        setUploadPath(path);
        appendLog(
          "info",
          "sftp",
          `upload prompt opened, title=${tab.title}, remoteDirectory=${workingDirectory || "/"}`,
        );
      } catch (err: unknown) {
        const message = errorMessage(err);
        appendLog("error", "sftp", `failed to resolve upload directory: ${message}`);
        setError(message);
      }
    },
    [appendLog],
  );

  const openRemoteDownload = useCallback(
    async (tab: TerminalTab) => {
      setTerminalMenu(null);
      setError("");
      if (!tab.sessionId) {
        setError("연결된 터미널 탭에서만 가져오기를 사용할 수 있습니다.");
        appendLog("warn", "sftp", "remote download skipped, no active SSH session");
        return;
      }
      try {
        appendLog(
          "info",
          "sftp",
          `remote download dialog requested, title=${tab.title}`,
        );
        const [workingDirectory, localDirectory] = await Promise.all([
          getSSHWorkingDirectory(tab.sessionId),
          defaultLocalDirectory(),
        ]);
        const remotePath = workingDirectory || "/";
        if (await shouldUseTerminalFileMode(tab)) {
          setRemoteDownload({
            entries: [],
            loading: true,
            localDirectory,
            mode: "terminal",
            overwrite: false,
            remotePath,
            selectedPaths: new Set(),
            sessionId: tab.sessionId,
            tabId: tab.id,
          });
          const entries = await listRemoteDirectoryViaTerminal(
            tab.sessionId,
            remotePath,
          );
          setRemoteDownload((current) =>
            current?.sessionId === tab.sessionId
              ? { ...current, entries, loading: false }
              : current,
          );
          setStatus("현재 터미널 계정 기준으로 원격 목록을 표시합니다.");
          appendLog(
            "info",
            "sftp",
            `terminal remote list loaded, path=${remotePath}, count=${entries.length}`,
          );
          return;
        }

        let sessionId = "";
        try {
          const session = await startSFTPSession({
            ssh: {
              profile: tab.profile,
              password: tab.password,
              passphrase: tab.passphrase,
              columns: 80,
              rows: 24,
              timeoutSeconds: 15,
            },
          });
          sessionId = session.sessionId;
          setRemoteDownload({
            entries: [],
            loading: true,
            localDirectory,
            mode: "sftp",
            overwrite: false,
            remotePath,
            selectedPaths: new Set(),
            sessionId,
            tabId: tab.id,
          });
          const entries = await listRemoteDirectory(sessionId, remotePath);
          setRemoteDownload((current) =>
            current?.sessionId === sessionId
              ? { ...current, entries, loading: false }
              : current,
          );
          appendLog(
            "info",
            "sftp",
            `sftp remote list loaded, path=${remotePath}, count=${entries.length}`,
          );
        } catch (sftpErr: unknown) {
          if (sessionId) {
            closeSFTPSession(sessionId).catch(() => undefined);
          }
          setRemoteDownload({
            entries: [],
            loading: true,
            localDirectory,
            mode: "terminal",
            overwrite: false,
            remotePath,
            selectedPaths: new Set(),
            sessionId: tab.sessionId,
            tabId: tab.id,
          });
          const entries = await listRemoteDirectoryViaTerminal(
            tab.sessionId,
            remotePath,
          );
          setRemoteDownload((current) =>
            current?.sessionId === tab.sessionId
              ? { ...current, entries, loading: false }
              : current,
          );
          setStatus(`SFTP 실패로 터미널 계정 기준 목록을 표시합니다: ${errorMessage(sftpErr)}`);
          appendLog(
            "warn",
            "sftp",
            `sftp remote list failed, terminal list loaded, path=${remotePath}, count=${entries.length}: ${errorMessage(sftpErr)}`,
          );
        }
      } catch (err: unknown) {
        const message = errorMessage(err);
        appendLog("error", "sftp", `remote download dialog failed: ${message}`);
        setError(message);
      }
    },
    [appendLog],
  );

  const closeRemoteDownload = useCallback(() => {
    if (remoteDownload?.mode === "sftp") {
      closeSFTPSession(remoteDownload.sessionId).catch(() => undefined);
    }
    setRemoteDownload(null);
  }, [remoteDownload]);

  const loadRemoteDirectory = useCallback(
    async (remotePath: string) => {
      if (!remoteDownload) {
        return;
      }
      setRemoteDownload({
        ...remoteDownload,
        loading: true,
        remotePath,
        selectedPaths: new Set(),
      });
      try {
      const entries =
          remoteDownload.mode === "terminal"
            ? await listRemoteDirectoryViaTerminal(
                remoteDownload.sessionId,
                remotePath,
              )
            : await listRemoteDirectory(remoteDownload.sessionId, remotePath);
        setRemoteDownload((current) =>
          current?.sessionId === remoteDownload.sessionId
            ? {
                ...current,
                entries,
                loading: false,
                remotePath,
              selectedPaths: new Set(),
            }
            : current,
        );
        appendLog(
          "info",
          "sftp",
          `remote directory loaded, mode=${remoteDownload.mode}, path=${remotePath}, count=${entries.length}`,
        );
      } catch (err: unknown) {
        setRemoteDownload((current) =>
          current?.sessionId === remoteDownload.sessionId
            ? { ...current, loading: false }
            : current,
        );
        const message = errorMessage(err);
        appendLog(
          "error",
          "sftp",
          `remote directory load failed, mode=${remoteDownload.mode}, path=${remotePath}: ${message}`,
        );
        setError(message);
      }
    },
    [appendLog, remoteDownload],
  );

  const downloadSelectedRemoteFiles = useCallback(async () => {
    if (!remoteDownload) {
      return;
    }
    setError("");
    try {
      appendLog(
        "info",
        "sftp",
        `download requested, mode=${remoteDownload.mode}, count=${remoteDownload.selectedPaths.size}, localDirectory=${remoteDownload.localDirectory}`,
      );
      const transfers = await Promise.all(
        Array.from(remoteDownload.selectedPaths).map(async (remotePath) => {
          if (remoteDownload.mode === "terminal") {
            return downloadFileViaTerminal({
              sshSessionId: remoteDownload.sessionId,
              localPath: remoteDownload.localDirectory,
              remotePath,
              overwrite: remoteDownload.overwrite,
            });
          }
          try {
            return await downloadFile({
              sessionId: remoteDownload.sessionId,
              localPath: remoteDownload.localDirectory,
              remotePath,
              overwrite: remoteDownload.overwrite,
            });
          } catch (sftpErr: unknown) {
            const tab = terminalTabsRef.current.find(
              (item) => item.id === remoteDownload.tabId,
            );
            if (!tab?.sessionId) {
              throw sftpErr;
            }
            setStatus(
              `SFTP 실패로 터미널 계정 기준 다운로드 중: ${errorMessage(sftpErr)}`,
            );
            appendLog(
              "warn",
              "sftp",
              `download fallback to terminal, remote=${remotePath}: ${errorMessage(sftpErr)}`,
            );
            return downloadFileViaTerminal({
              sshSessionId: tab.sessionId,
              localPath: remoteDownload.localDirectory,
              remotePath,
              overwrite: remoteDownload.overwrite,
            });
          }
        }),
      );
      const transferIds = transfers
        .map((transfer) => transfer.transferId)
        .filter((transferId) => transferId);
      if (transferIds.length > 0) {
        const batchId = crypto.randomUUID();
        downloadOpenBatchRef.current.set(batchId, {
          completed: 0,
          directory: remoteDownload.localDirectory,
          files: [],
          remaining: new Set(transferIds),
        });
        for (const transferId of transferIds) {
          downloadTransferBatchRef.current.set(transferId, batchId);
        }
        appendLog(
          "info",
          "sftp",
          `download transfers started, count=${transferIds.length}`,
        );
      }
    } catch (err: unknown) {
      const message = errorMessage(err);
      appendLog("error", "sftp", `download failed: ${message}`);
      setError(message);
    }
  }, [appendLog, remoteDownload]);

  const browseDownloadDirectory = useCallback(async () => {
    if (!remoteDownload) {
      return;
    }
    try {
      const selected = await selectLocalDirectory(remoteDownload.localDirectory);
      if (!selected) {
        return;
      }
      setRemoteDownload((current) =>
        current ? { ...current, localDirectory: selected } : current,
      );
      appendLog("info", "sftp", `download local directory selected: ${selected}`);
    } catch (err: unknown) {
      const message = errorMessage(err);
      appendLog("error", "sftp", `download local directory selection failed: ${message}`);
      setError(message);
    }
  }, [appendLog, remoteDownload]);

  const handleAppFontWheel = useCallback((event: WheelEvent<HTMLElement>) => {
    if (!event.ctrlKey) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const delta = event.deltaY < 0 ? 1 : -1;
    setAppearance((current) => ({
      ...current,
      appFontSize: clampNumber(current.appFontSize + delta, 12, 20),
    }));
  }, []);

  const handleTerminalFontWheel = useCallback((deltaY: number) => {
    const delta = deltaY < 0 ? 1 : -1;
    setAppearance((current) => ({
      ...current,
      terminalFontSize: clampNumber(current.terminalFontSize + delta, 10, 28),
    }));
  }, []);

  return (
    <main
      className={[
        "app-shell",
        leftPanelCollapsed ? "left-panel-collapsed" : "",
        rightPanelCollapsed ? "right-panel-collapsed" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      data-theme={appearance.theme}
      style={{
        "--left-panel-width": leftPanelCollapsed
          ? "0px"
          : `${leftPanelWidth}px`,
        "--right-panel-width": rightPanelCollapsed
          ? "0px"
          : `${rightPanelWidth}px`,
        fontFamily: appearance.appFontFamily,
        fontSize: `${appearance.appFontSize}px`,
      } as CSSProperties}
    >
      <aside className="sidebar" onWheelCapture={handleAppFontWheel}>
        <div className="app-header">
          <div>
            <h1>LightSSH</h1>
            <p>{activeTab?.message ?? status}</p>
          </div>
          <button
            type="button"
            className="icon-button"
            onClick={() => setSettingsOpen(true)}
            title="옵션"
          >
            옵션
          </button>
        </div>
        <div className="connection-form">
          <label>
            인증 방식
            <select
              value={profile.authType}
              onChange={(event) => {
                const authType = event.target.value as ConnectionProfile["authType"];
                updateProfileDraft({ authType });
                if (authType !== "password") {
                  setPasswordSaved(false);
                }
              }}
            >
              <option value="password">비밀번호</option>
              <option value="private_key">개인키</option>
              <option value="ssh_agent">ssh-agent</option>
            </select>
          </label>
          <label>
            프로필 이름
            <input
              value={profile.name}
              onChange={(event) =>
                updateProfileDraft({ name: event.target.value })
              }
              placeholder="운영 서버"
            />
          </label>
          <label>
            호스트
            <input
              value={profile.host}
              onChange={(event) =>
                updateProfileDraft({ host: event.target.value })
              }
              placeholder="server.example.internal"
            />
          </label>
          <label>
            포트
            <input
              type="number"
              min="1"
              max="65535"
              value={profile.port}
              onChange={(event) =>
                updateProfileDraft({ port: Number(event.target.value) })
              }
            />
          </label>
          <label>
            사용자명
            <input
              value={profile.username}
              onChange={(event) =>
                updateProfileDraft({ username: event.target.value })
              }
              placeholder="ops"
            />
          </label>
          {profile.authType === "password" ? (
            <>
              <label>
                비밀번호
                <input
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={passwordSaved ? "저장된 비밀번호 사용" : ""}
                />
              </label>
              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={passwordSaved}
                  onChange={(event) => setPasswordSaved(event.target.checked)}
                />
                비밀번호 저장
              </label>
            </>
          ) : null}
          {profile.authType === "private_key" ? (
            <>
              <label>
                개인키 경로
                <input
                  value={profile.keyPath ?? ""}
                  onChange={(event) =>
                    updateProfileDraft({ keyPath: event.target.value })
                  }
                  placeholder="C:/Users/user/.ssh/id_ed25519"
                />
              </label>
              <label>
                Passphrase
                <input
                  type="password"
                  value={passphrase}
                  onChange={(event) => setPassphrase(event.target.value)}
                />
              </label>
            </>
          ) : null}
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={sessionKeepAlive}
              onChange={(event) => setSessionKeepAlive(event.target.checked)}
            />
            세션유지
          </label>
          <div className="button-row">
            <button type="button" onClick={connect}>
              연결
            </button>
            <button
              type="button"
              className="secondary-button"
              onClick={disconnectActive}
              disabled={!activeTab?.sessionId}
            >
              종료
            </button>
          </div>
          <button type="button" className="secondary-button" onClick={saveCurrentProfile}>
            프로필 저장
          </button>
          {error ? <div className="error-box">{error}</div> : null}
        </div>
      </aside>
      <div
        className="panel-resizer left-panel-resizer"
        role="separator"
        title="좌측 패널 크기 조절"
        onPointerDown={(event) => startPanelResize("left", event)}
      />
      <section className="workspace">
        <div className="workspace-tabs">
          <div className="workspace-tab-group">
            <button
              type="button"
              className="panel-toggle-button"
              aria-pressed={leftPanelCollapsed}
              title={
                leftPanelCollapsed ? "좌측 접속 패널 펼치기" : "좌측 접속 패널 접기"
              }
              onClick={() => {
                setLeftPanelCollapsed((current) => !current);
                setTerminalFocusKey((current) => current + 1);
              }}
            >
              <span className="panel-toggle-icon" aria-hidden="true">
                {leftPanelCollapsed ? "☰" : "◧"}
              </span>
            </button>
            <button
              type="button"
              className={activeView === "terminal" ? "tab-button active" : "tab-button"}
              onClick={() => {
                setActiveView("terminal");
                setTerminalFocusKey((current) => current + 1);
              }}
            >
              터미널
            </button>
            <button
              type="button"
              className={activeView === "tunnel" ? "tab-button active" : "tab-button"}
              onClick={() => setActiveView("tunnel")}
            >
              터널
            </button>
            <button
              type="button"
              className={activeView === "log" ? "tab-button active" : "tab-button"}
              onClick={() => setActiveView("log")}
            >
              로그
            </button>
          </div>
          <div className="workspace-help-group">
            <button
              type="button"
              className="tab-button help-button"
              onClick={() => setHelpOpen(true)}
            >
              도움말
            </button>
            <button
              type="button"
              className="panel-toggle-button"
              aria-pressed={rightPanelCollapsed}
              title={
                rightPanelCollapsed ? "우측 프로필 패널 펼치기" : "우측 프로필 패널 접기"
              }
              onClick={() => {
                setRightPanelCollapsed((current) => !current);
                setTerminalFocusKey((current) => current + 1);
              }}
            >
              <span className="panel-toggle-icon" aria-hidden="true">
                {rightPanelCollapsed ? "☰" : "◨"}
              </span>
            </button>
          </div>
        </div>
        <div
          className={
            activeView === "terminal" ? "view-pane active" : "view-pane hidden"
          }
        >
          <div className="terminal-tabbar">
            {terminalTabs.length === 0 ? (
              <span>열린 터미널이 없습니다.</span>
            ) : (
              terminalTabs.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  className={
                    tab.id === activeTabId
                      ? "terminal-tab active"
                      : "terminal-tab"
                  }
                  onClick={() => {
                    activateTerminalTab(tab);
                  }}
                >
                  <span className={`status-dot ${tab.status}`} />
                  <span>{tab.title}</span>
                  <span
                    className="tab-close"
                    role="button"
                    tabIndex={0}
                    onClick={(event) => {
                      event.stopPropagation();
                      closeTab(tab);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        event.stopPropagation();
                        closeTab(tab);
                      }
                    }}
                  >
                    x
                  </span>
                </button>
              ))
            )}
          </div>
          <div className="terminal-stack">
            {terminalTabs.map((tab) => (
              <div
                key={tab.id}
                className={
                  tab.id === activeTabId
                    ? "terminal-layer active"
                    : "terminal-layer hidden"
                }
              >
                <TerminalPanel
                  active={tab.id === activeTabId && activeView === "terminal"}
                  disabled={!tab.sessionId}
                  focusKey={tab.id === activeTabId ? terminalFocusKey : 0}
                  fontFamily={appearance.terminalFontFamily}
                  fontSize={appearance.terminalFontSize}
                  theme={appearance.theme}
                  onData={(data) => sendInput(tab, data)}
                  onContextMenu={(x, y, selection) => {
                    activateTerminalTab(tab);
                    appendLog(
                      "info",
                      "terminal",
                      `context menu opened, title=${tab.title}, selectionLength=${selection.length}`,
                    );
                    setTerminalMenu({
                      selection,
                      tabId: tab.id,
                      x: clampNumber(x, 8, window.innerWidth - 190),
                      y: clampNumber(y, 8, window.innerHeight - 128),
                    });
                  }}
                  onCopy={copyTerminalSelection}
                  onDiagnostic={(message) =>
                    appendLog("info", "terminal", `${tab.title}: ${message}`)
                  }
                  onFileDrop={(path) => handleTerminalFileDrop(tab, path)}
                  onPaste={() => pasteClipboardToTab(tab)}
                  onZoom={handleTerminalFontWheel}
                  onResize={(columns, rows) =>
                    resizeTerminal(tab.id, columns, rows)
                  }
                  registerWriter={(writer) => registerTabWriter(tab.id, writer)}
                />
              </div>
            ))}
          </div>
        </div>
        <div
          className={
            activeView === "tunnel" ? "view-pane active" : "view-pane hidden"
          }
        >
          <TunnelPanel
            profile={profile}
            password={password}
            passphrase={passphrase}
          />
        </div>
        <div
          className={
            activeView === "log" ? "view-pane active" : "view-pane hidden"
          }
        >
          <LogPanel
            logs={logs}
            onClear={() => setLogs([])}
            onCopy={copyLogs}
            onReveal={revealLogs}
          />
        </div>
      </section>
      <div
        className="panel-resizer right-panel-resizer"
        role="separator"
        title="우측 패널 크기 조절"
        onPointerDown={(event) => startPanelResize("right", event)}
      />
      <aside className="profiles-panel" onWheelCapture={handleAppFontWheel}>
        <h2>프로필</h2>
        <div className="profile-list">
          {profiles.length === 0 ? (
            <div className="empty-state">저장된 프로필이 없습니다.</div>
          ) : (
            profiles.map((savedProfile) => (
              <ConnectionProfileCard
                key={savedProfile.id}
                profile={savedProfile}
                onEdit={() => selectProfile(savedProfile)}
                onDelete={async () => {
                  if (!window.confirm(`${savedProfile.name} 프로필을 삭제할까요?`)) {
                    return;
                  }
                  await deleteProfile(savedProfile.id);
                  const nextProfiles = await loadProfiles();
                  setProfiles(nextProfiles);
                  if (profile.id === savedProfile.id) {
                    setProfile(emptyProfile);
                    setPassword("");
                    setPasswordSaved(false);
                  }
                }}
              />
            ))
          )}
        </div>
        {Object.values(transfers).length > 0 ? (
          <div className="transfer-panel">
            <h3>전송</h3>
            {Object.values(transfers)
              .slice(-5)
              .map((transfer) => (
                <div key={transfer.transferId} className="transfer-compact-row">
                  <div className="transfer-compact-title">
                    <strong>{transferFileName(transfer)}</strong>
                    <span>{transferDirectionLabel(transfer)}</span>
                  </div>
                  <progress
                    value={transfer.total > 0 ? transfer.bytes : transferPercent(transfer)}
                    max={transfer.total > 0 ? transfer.total : 100}
                  />
                  <div className="transfer-compact-meta">
                    <span>{transferStatusLabel(transfer.status)}</span>
                    <span>{transferPercent(transfer)}%</span>
                  </div>
                  <div className="transfer-speed-row">
                    <span>
                      현재 {formatSpeed(transferMetrics[transfer.transferId]?.speedBytesPerSecond ?? 0)}
                    </span>
                    <span>
                      평균 {formatSpeed(transferMetrics[transfer.transferId]?.averageBytesPerSecond ?? 0)}
                    </span>
                  </div>
                  <div className="transfer-size-row">
                    {formatBytes(transfer.bytes)} / {formatBytes(transfer.total)}
                  </div>
                </div>
              ))}
          </div>
        ) : null}
      </aside>
      <HostKeyModal
        prompt={hostKeyPrompt}
        onDecision={async (accept) => {
          if (!hostKeyPrompt) {
            return;
          }
          try {
            await resolveHostKeyPrompt({
              requestId: hostKeyPrompt.requestId,
              accept,
            });
            setHostKeyPrompt(null);
            if (accept) {
              setStatus("호스트 키 저장됨. 다시 연결 중...");
              await connect();
            } else {
              setStatus("연결 실패");
              setError("호스트 키 승인이 취소되어 연결하지 않았습니다.");
            }
          } catch (err: unknown) {
            setHostKeyPrompt(null);
            setError(errorMessage(err));
          }
        }}
      />
      <UploadConfirmModal
        localPath={uploadPath}
        remotePath={remoteUploadPath}
        transferMode={uploadTransferMode}
        onRemotePathChange={setRemoteUploadPath}
        onTransferModeChange={setUploadTransferMode}
        onOverwrite={() => {
          if (uploadPath) {
            uploadDroppedFile(uploadPath, true);
          }
        }}
        onRename={() => {
          if (!uploadPath) {
            return;
          }
          const nextName = window.prompt("새 파일명", baseName(uploadPath));
          if (nextName) {
            uploadDroppedFile(uploadPath, false, nextName);
          }
        }}
        onCancel={() => setUploadPath(null)}
      />
      {settingsOpen ? (
        <SettingsModal
          settings={appearance}
          onChange={setAppearance}
          onClose={() => setSettingsOpen(false)}
        />
      ) : null}
      {helpOpen ? <HelpModal onClose={() => setHelpOpen(false)} /> : null}
      {terminalMenu ? (
        <div
          className="terminal-context-menu"
          style={{ left: terminalMenu.x, top: terminalMenu.y }}
          onClick={(event) => event.stopPropagation()}
          onContextMenu={(event) => event.preventDefault()}
          onPointerDown={(event) => event.stopPropagation()}
        >
          <button
            type="button"
            disabled={!terminalMenu.selection.trim()}
            onClick={() => {
              copyTerminalSelection(terminalMenu.selection);
              setTerminalMenu(null);
            }}
          >
            복사
          </button>
          <button
            type="button"
            onClick={() => {
              const tab =
                terminalTabs.find((item) => item.id === terminalMenu.tabId) ??
                null;
              pasteClipboardToTab(tab);
              setTerminalMenu(null);
            }}
          >
            붙여넣기
          </button>
          <button
            type="button"
            onClick={() => {
              const tab = terminalTabs.find(
                (item) => item.id === terminalMenu.tabId,
              );
              if (tab) {
                openRemoteDownload(tab);
              }
              setTerminalMenu(null);
            }}
          >
            내 PC로 가져오기
          </button>
        </div>
      ) : null}
      {remoteDownload ? (
        <RemoteDownloadModal
          entries={remoteDownload.entries}
          loading={remoteDownload.loading}
          localDirectory={remoteDownload.localDirectory}
          overwrite={remoteDownload.overwrite}
          remotePath={remoteDownload.remotePath}
          selectedPaths={remoteDownload.selectedPaths}
          onClose={closeRemoteDownload}
          onDownload={downloadSelectedRemoteFiles}
          onBrowseLocalDirectory={browseDownloadDirectory}
          onLocalDirectoryChange={(localDirectory) =>
            setRemoteDownload((current) =>
              current ? { ...current, localDirectory } : current,
            )
          }
          onOverwriteChange={(overwrite) =>
            setRemoteDownload((current) =>
              current ? { ...current, overwrite } : current,
            )
          }
          onRefresh={() => loadRemoteDirectory(remoteDownload.remotePath)}
          onSelect={(path, selected) =>
            setRemoteDownload((current) => {
              if (!current) {
                return current;
              }
              const selectedPaths = new Set(current.selectedPaths);
              if (selected) {
                selectedPaths.add(path);
              } else {
                selectedPaths.delete(path);
              }
              return { ...current, selectedPaths };
            })
          }
        />
      ) : null}
    </main>
  );
}

function LogPanel({
  logs,
  onClear,
  onCopy,
  onReveal,
}: {
  logs: AppLogEntry[];
  onClear: () => void;
  onCopy: () => void;
  onReveal: () => void;
}) {
  return (
    <section className="log-panel">
      <header className="log-toolbar">
        <div>
          <h2>동작 로그</h2>
          <p>
            SSH, SFTP, 터널, 클립보드 이벤트를 표시하고 날짜별 파일에도 저장합니다.
          </p>
        </div>
        <div className="log-actions">
          <button type="button" className="secondary-button" onClick={onCopy}>
            로그 복사
          </button>
          <button type="button" className="secondary-button" onClick={onReveal}>
            탐색기 열기
          </button>
          <button type="button" className="secondary-button" onClick={onClear}>
            지우기
          </button>
        </div>
      </header>
      <div className="log-list">
        {logs.length === 0 ? (
          <div className="empty-state">아직 로그가 없습니다.</div>
        ) : (
          logs
            .slice()
            .reverse()
            .map((entry) => (
              <article key={entry.id} className={`log-row ${entry.level}`}>
                <span>{entry.at}</span>
                <strong>{entry.source}</strong>
                <em>{entry.level}</em>
                <p>{entry.message}</p>
              </article>
            ))
        )}
      </div>
    </section>
  );
}

function tabTitle(profile: ConnectionProfile): string {
  const name = profile.name.trim();
  if (name && name !== "수동 접속") {
    return name;
  }
  return profile.host || "터미널";
}

async function shouldUseTerminalFileMode(tab: TerminalTab): Promise<boolean> {
  if (!tab.sessionId) {
    return false;
  }
  try {
    const remoteUser = (await getSSHRemoteUser(tab.sessionId)).trim();
    return Boolean(remoteUser) && remoteUser !== tab.profile.username;
  } catch {
    return false;
  }
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function formatLogTimestamp(date: Date): string {
  const pad = (value: number) => value.toString().padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(
    date.getSeconds(),
  )}`;
}

function formatLogEntries(logs: AppLogEntry[]): string {
  return logs.map(formatLogEntry).join("\n");
}

function formatLogEntry(entry: AppLogEntry): string {
  return `[${entry.at}] ${entry.level.toUpperCase()} ${entry.source}: ${entry.message}`;
}

function sanitizeLogMessage(message: string): string {
  return message
    .replace(/(password|passphrase|token|secret)=([^,\s]+)/gi, "$1=***")
    .replace(/(password|passphrase|token|secret):\s*([^,\s]+)/gi, "$1: ***");
}

function baseName(path: string): string {
  return path.split(/[\\/]/).filter(Boolean).pop() ?? "upload.bin";
}

function joinRemotePath(dir: string, name: string): string {
  const base = dir.endsWith("/") ? dir.slice(0, -1) : dir;
  return `${base || "/"}/${name}`.replace(/\/+/g, "/");
}

function normalizeTerminalPaste(text: string): string {
  return text.replace(/\r\n/g, "\r").replace(/\n/g, "\r");
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function isFinishedTransfer(status: TransferProgressEvent["status"]): boolean {
  return status === "completed" || status === "failed" || status === "canceled";
}

function transferPercent(transfer: TransferProgressEvent): number {
  if (transfer.status === "completed") {
    return 100;
  }
  if (transfer.total <= 0) {
    return transfer.status === "started" ? 0 : 100;
  }
  return Math.min(100, Math.round((transfer.bytes / transfer.total) * 100));
}

function transferFileName(transfer: TransferProgressEvent): string {
  const path =
    transfer.direction === "download" ? transfer.remotePath : transfer.localPath;
  return baseName(path);
}

function transferDirectionLabel(transfer: TransferProgressEvent): string {
  const direction = transfer.direction === "download" ? "다운로드" : "업로드";
  return transfer.transferId.startsWith("terminal:")
    ? `${direction} · 터미널`
    : direction;
}

function transferStatusLabel(status: TransferProgressEvent["status"]): string {
  switch (status) {
    case "started":
      return "시작";
    case "in_progress":
      return "진행 중";
    case "completed":
      return "완료";
    case "failed":
      return "실패";
    case "canceled":
      return "취소됨";
  }
}

function formatSpeed(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return "-";
  }
  return `${formatBytes(bytesPerSecond)}/s`;
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const digits = value >= 10 || unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[unitIndex]}`;
}
