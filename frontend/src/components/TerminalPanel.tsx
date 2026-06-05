import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useCallback, useEffect, useRef } from "react";
import type { AppTheme } from "../api/settings";
import { onNativeFileDrop } from "../api/wails";

type TerminalPanelProps = {
  active: boolean;
  disabled: boolean;
  focusKey: number;
  fontFamily: string;
  fontSize: number;
  theme: AppTheme;
  onData: (data: string) => void;
  onContextMenu: (x: number, y: number, selection: string) => void;
  onCopy: (selection: string) => void;
  onDiagnostic: (message: string) => void;
  onFileDrop: (path: string) => void;
  onPaste: () => void;
  onResize: (columns: number, rows: number) => void;
  onZoom: (deltaY: number) => void;
  registerWriter: (writer: (data: string) => void) => void;
};

export function TerminalPanel({
  active,
  disabled,
  focusKey,
  fontFamily,
  fontSize,
  theme,
  onData,
  onContextMenu,
  onCopy,
  onDiagnostic,
  onFileDrop,
  onPaste,
  onResize,
  onZoom,
  registerWriter,
}: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const disabledRef = useRef(disabled);
  const onDataRef = useRef(onData);
  const onCopyRef = useRef(onCopy);
  const onDiagnosticRef = useRef(onDiagnostic);
  const onPasteRef = useRef(onPaste);
  const onResizeRef = useRef(onResize);
  const onZoomRef = useRef(onZoom);
  const registerWriterRef = useRef(registerWriter);
  const rightClickSelectionRef = useRef("");
  const lastDropRef = useRef({ path: "", at: 0 });

  useEffect(() => {
    disabledRef.current = disabled;
  }, [disabled]);

  useEffect(() => {
    onDataRef.current = onData;
    onCopyRef.current = onCopy;
    onDiagnosticRef.current = onDiagnostic;
    onPasteRef.current = onPaste;
    onResizeRef.current = onResize;
    onZoomRef.current = onZoom;
    registerWriterRef.current = registerWriter;
  }, [onCopy, onData, onDiagnostic, onPaste, onResize, onZoom, registerWriter]);

  const emitFileDrop = useCallback(
    (path: string) => {
      const now = Date.now();
      if (
        lastDropRef.current.path === path &&
        now - lastDropRef.current.at < 750
      ) {
        return;
      }
      lastDropRef.current = { path, at: now };
      onFileDrop(path);
    },
    [onFileDrop],
  );

  const fitAndNotify = useCallback(() => {
    fitAddonRef.current?.fit();
    const terminal = terminalRef.current;
    if (terminal) {
      onResizeRef.current(terminal.cols, terminal.rows);
    }
  }, []);

  const liveSelection = useCallback(() => {
    return terminalRef.current?.getSelection() ?? "";
  }, []);

  const copySelection = useCallback(() => {
    const selection = liveSelection();
    if (selection.trim()) {
      onDiagnosticRef.current(
        `keyboard copy requested, selectionLength=${selection.length}`,
      );
      onCopyRef.current(selection);
      return true;
    }
    return false;
  }, [liveSelection]);

  const pasteText = useCallback((text: string) => {
    if (disabledRef.current || !text) {
      return false;
    }
    onDataRef.current(normalizePastedText(text));
    terminalRef.current?.focus();
    return true;
  }, []);

  const pasteFromClipboard = useCallback(() => {
    terminalRef.current?.focus();
    onPasteRef.current();
  }, []);

  useEffect(() => {
    if (!active) {
      return () => undefined;
    }
    return onNativeFileDrop((_x, _y, paths) => {
      if (paths[0]) {
        emitFileDrop(paths[0]);
      }
    });
  }, [active, emitFileDrop]);

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const container = containerRef.current;
    const terminal = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontFamily,
      fontSize,
      theme: terminalTheme(theme),
    });
    terminalRef.current = terminal;
    const fitAddon = new FitAddon();
    fitAddonRef.current = fitAddon;
    terminal.loadAddon(fitAddon);
    terminal.open(container);
    terminal.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown") {
        return true;
      }
      const key = event.key.toLowerCase();
      if ((event.ctrlKey || event.metaKey) && key === "v") {
        event.preventDefault();
        pasteFromClipboard();
        return false;
      }
      if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === "c") {
        if (copySelection()) {
          event.preventDefault();
          return false;
        }
      }
      return true;
    });

    const dataDisposable = terminal.onData((data) => {
      if (!disabledRef.current) {
        onDataRef.current(data);
      }
    });
    const resizeDisposable = terminal.onResize(({ cols, rows }) => {
      onResizeRef.current(cols, rows);
    });
    registerWriterRef.current((data) => terminal.write(data));
    fitAndNotify();
    terminal.focus();

    const resizeObserver = new ResizeObserver(() => {
      if (containerRef.current?.offsetParent !== null) {
        fitAndNotify();
      }
    });
    resizeObserver.observe(container);

    const handleWheel = (event: globalThis.WheelEvent) => {
      if (!event.ctrlKey) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      onZoomRef.current(event.deltaY);
    };
    container.addEventListener("wheel", handleWheel, {
      capture: true,
      passive: false,
    });
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      const key = event.key.toLowerCase();
      if ((event.ctrlKey || event.metaKey) && key === "v") {
        event.preventDefault();
        event.stopPropagation();
        pasteFromClipboard();
        return;
      }
      if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === "c") {
        if (copySelection()) {
          event.preventDefault();
          event.stopPropagation();
        }
      }
    };
    const handlePaste = (event: ClipboardEvent) => {
      const text = event.clipboardData?.getData("text/plain") ?? "";
      onDiagnosticRef.current(`dom paste event, textLength=${text.length}`);
      if (pasteText(text)) {
        event.preventDefault();
        event.stopPropagation();
      }
    };
    const handleCopy = (event: ClipboardEvent) => {
      const selection = liveSelection();
      if (!selection.trim()) {
        return;
      }
      event.clipboardData?.setData("text/plain", selection);
      event.preventDefault();
      event.stopPropagation();
      onDiagnosticRef.current(
        `dom copy event handled, selectionLength=${selection.length}`,
      );
      onCopyRef.current(selection);
    };
    container.addEventListener("keydown", handleKeyDown, { capture: true });
    container.addEventListener("paste", handlePaste, { capture: true });
    container.addEventListener("copy", handleCopy, { capture: true });

    return () => {
      container.removeEventListener("wheel", handleWheel, { capture: true });
      container.removeEventListener("keydown", handleKeyDown, { capture: true });
      container.removeEventListener("paste", handlePaste, { capture: true });
      container.removeEventListener("copy", handleCopy, { capture: true });
      resizeObserver.disconnect();
      dataDisposable.dispose();
      resizeDisposable.dispose();
      registerWriterRef.current(() => undefined);
      fitAddonRef.current = null;
      terminalRef.current = null;
      terminal.dispose();
    };
  }, [copySelection, fitAndNotify, liveSelection, pasteFromClipboard, pasteText]);

  useEffect(() => {
    const terminal = terminalRef.current;
    if (!terminal) {
      return;
    }
    terminal.options.fontFamily = fontFamily;
    terminal.options.fontSize = fontSize;
    terminal.options.theme = terminalTheme(theme);
    fitAndNotify();
  }, [fitAndNotify, fontFamily, fontSize, theme]);

  useEffect(() => {
    if (!active) {
      return;
    }
    const handle = window.setTimeout(() => {
      fitAndNotify();
      terminalRef.current?.focus();
    }, 0);
    return () => window.clearTimeout(handle);
  }, [active, fitAndNotify, focusKey]);

  return (
    <div
      ref={containerRef}
      className="terminal-panel"
      onDragEnterCapture={(event) => event.preventDefault()}
      onPointerDownCapture={(event) => {
        if (event.button !== 2) {
          return;
        }
        rightClickSelectionRef.current = liveSelection();
        onDiagnosticRef.current(
          `right pointer down, selectionLength=${rightClickSelectionRef.current.length}`,
        );
        terminalRef.current?.focus();
      }}
      onContextMenu={(event) => {
        event.preventDefault();
        terminalRef.current?.focus();
        const selection = rightClickSelectionRef.current || liveSelection();
        rightClickSelectionRef.current = "";
        onDiagnosticRef.current(
          `context menu event, shift=${event.shiftKey}, selectionLength=${selection.length}`,
        );
        if (event.shiftKey) {
          onDiagnosticRef.current("opening terminal context menu");
          onContextMenu(event.clientX, event.clientY, selection);
          return;
        }
        if (selection.trim()) {
          onDiagnosticRef.current(
            `right-click copy requested, selectionLength=${selection.length}`,
          );
          onCopyRef.current(selection);
          terminalRef.current?.clearSelection();
          terminalRef.current?.focus();
          return;
        }
        onDiagnosticRef.current("right-click paste requested");
        pasteFromClipboard();
      }}
      onDragOverCapture={(event) => {
        event.preventDefault();
        event.dataTransfer.dropEffect = "copy";
      }}
      onDropCapture={(event) => {
        event.preventDefault();
        const file = event.dataTransfer.files[0] as
          | (File & { path?: string })
          | undefined;
        const fallback = event.dataTransfer.getData("text/plain");
        if (file?.path || fallback) {
          emitFileDrop(file?.path ?? fallback);
        }
      }}
    />
  );
}

function terminalTheme(theme: AppTheme) {
  if (theme === "light") {
    return {
      background: "#ffffff",
      foreground: "#1d252f",
      cursor: "#1d252f",
      selectionBackground: "#cde0ff",
    };
  }
  return {
    background: "#101820",
    foreground: "#e6edf3",
    cursor: "#ffffff",
    selectionBackground: "#385170",
  };
}

function normalizePastedText(text: string): string {
  return text.replace(/\r\n/g, "\r").replace(/\n/g, "\r");
}
