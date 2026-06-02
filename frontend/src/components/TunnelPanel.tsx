import { useCallback, useEffect, useMemo, useState } from "react";
import type { ConnectionProfile } from "../api/profiles";
import {
  listTunnels,
  onTunnelEvent,
  startLocalTunnel,
  startReverseTunnel,
  stopLocalTunnel,
  type LocalTunnelConfig,
  type ReverseTunnelConfig,
  type TunnelInfo,
} from "../api/tunnel";

type TunnelPanelProps = {
  profile: ConnectionProfile;
  password: string;
  passphrase: string;
};

const defaultConfig: LocalTunnelConfig = {
  id: "",
  name: "Local tunnel",
  localBindAddress: "127.0.0.1",
  localPort: 8080,
  targetHost: "",
  targetPort: 80,
};

const defaultReverseConfig: ReverseTunnelConfig = {
  id: "",
  name: "Reverse tunnel",
  remoteBindAddress: "127.0.0.1",
  remotePort: 18080,
  localTargetHost: "127.0.0.1",
  localTargetPort: 8080,
};

export function TunnelPanel({ profile, password, passphrase }: TunnelPanelProps) {
  const [config, setConfig] = useState<LocalTunnelConfig>(defaultConfig);
  const [reverseConfig, setReverseConfig] =
    useState<ReverseTunnelConfig>(defaultReverseConfig);
  const [mode, setMode] = useState<"local" | "reverse">("local");
  const [tunnels, setTunnels] = useState<Record<string, TunnelInfo>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    listTunnels()
      .then((items) => {
        setTunnels(Object.fromEntries(items.map((item) => [item.tunnelId, item])));
      })
      .catch((err: unknown) => setError(errorMessage(err)));

    return onTunnelEvent((event) => {
      setTunnels((current) => {
        const existing = current[event.tunnelId];
        if (!existing) {
          return current;
        }
        if (event.status === "stopped") {
          const next = { ...current };
          next[event.tunnelId] = {
            ...existing,
            status: "stopped",
            connectionCount: 0,
            message: event.message,
          };
          return next;
        }
        return {
          ...current,
          [event.tunnelId]: {
            ...existing,
            status: event.status,
            connectionCount: event.connectionCount,
            message: event.message,
          },
        };
      });
      if (event.status === "error" && event.message) {
        setError(event.message);
      }
    });
  }, []);

  const tunnelList = useMemo(
    () =>
      Object.values(tunnels).sort((a, b) =>
        a.tunnelId.localeCompare(b.tunnelId),
      ),
    [tunnels],
  );

  const start = useCallback(async () => {
    setError("");
    const tunnelId = config.id || crypto.randomUUID();
    try {
      const info = await startLocalTunnel({
        ssh: {
          profile,
          password,
          passphrase,
          columns: 80,
          rows: 24,
          timeoutSeconds: 15,
        },
        config: {
          ...config,
          id: tunnelId,
          name: config.name || tunnelId,
        },
      });
      setTunnels((current) => ({ ...current, [info.tunnelId]: info }));
      setConfig({ ...config, id: "", name: config.name || "Local tunnel" });
    } catch (err: unknown) {
      setError(errorMessage(err));
    }
  }, [config, passphrase, password, profile]);

  const startReverse = useCallback(async () => {
    setError("");
    const tunnelId = reverseConfig.id || crypto.randomUUID();
    try {
      const info = await startReverseTunnel({
        ssh: {
          profile,
          password,
          passphrase,
          columns: 80,
          rows: 24,
          timeoutSeconds: 15,
        },
        config: {
          ...reverseConfig,
          id: tunnelId,
          name: reverseConfig.name || tunnelId,
        },
      });
      setTunnels((current) => ({ ...current, [info.tunnelId]: info }));
      setReverseConfig({
        ...reverseConfig,
        id: "",
        name: reverseConfig.name || "Reverse tunnel",
      });
    } catch (err: unknown) {
      setError(errorMessage(err));
    }
  }, [passphrase, password, profile, reverseConfig]);

  const stop = useCallback(async (tunnelId: string) => {
    try {
      await stopLocalTunnel(tunnelId);
      setTunnels((current) => {
        const existing = current[tunnelId];
        if (!existing) {
          return current;
        }
        return {
          ...current,
          [tunnelId]: { ...existing, status: "stopped", connectionCount: 0 },
        };
      });
    } catch (err: unknown) {
      setError(errorMessage(err));
    }
  }, []);

  return (
    <div className="tunnel-panel">
      <section className="tunnel-form">
        <header>
          <h2>Port Forwarding</h2>
          <p>
            Local은 로컬 listen 포트를 원격 target으로, Reverse는 SSH 서버의
            remote listen 포트를 로컬 target으로 전달합니다.
          </p>
        </header>
        <div className="mode-toggle">
          <button
            type="button"
            className={mode === "local" ? "tab-button active" : "tab-button"}
            onClick={() => setMode("local")}
          >
            Local
          </button>
          <button
            type="button"
            className={mode === "reverse" ? "tab-button active" : "tab-button"}
            onClick={() => setMode("reverse")}
          >
            Reverse
          </button>
        </div>
        {mode === "local" ? (
          <div className="tunnel-grid">
            <label>
              이름
              <input
                value={config.name}
                onChange={(event) =>
                  setConfig({ ...config, name: event.target.value })
                }
              />
            </label>
            <label>
              Local bind address
              <input
                value={config.localBindAddress}
                onChange={(event) =>
                  setConfig({ ...config, localBindAddress: event.target.value })
                }
              />
            </label>
            <label>
              Local port
              <input
                type="number"
                min="1"
                max="65535"
                value={config.localPort}
                onChange={(event) =>
                  setConfig({ ...config, localPort: Number(event.target.value) })
                }
              />
            </label>
            <label>
              Target host
              <input
                value={config.targetHost}
                onChange={(event) =>
                  setConfig({ ...config, targetHost: event.target.value })
                }
                placeholder="127.0.0.1 또는 db.internal"
              />
            </label>
            <label>
              Target port
              <input
                type="number"
                min="1"
                max="65535"
                value={config.targetPort}
                onChange={(event) =>
                  setConfig({ ...config, targetPort: Number(event.target.value) })
                }
              />
            </label>
            <button type="button" onClick={start}>
              Local 시작
            </button>
          </div>
        ) : (
          <>
            <div className="notice-box">
              GatewayPorts 서버 설정에 따라 remote bind address가
              127.0.0.1로 제한되거나 요청한 주소가 무시될 수 있습니다.
            </div>
            <div className="tunnel-grid">
              <label>
                이름
                <input
                  value={reverseConfig.name}
                  onChange={(event) =>
                    setReverseConfig({
                      ...reverseConfig,
                      name: event.target.value,
                    })
                  }
                />
              </label>
              <label>
                Remote bind address
                <input
                  value={reverseConfig.remoteBindAddress}
                  onChange={(event) =>
                    setReverseConfig({
                      ...reverseConfig,
                      remoteBindAddress: event.target.value,
                    })
                  }
                />
              </label>
              <label>
                Remote port
                <input
                  type="number"
                  min="1"
                  max="65535"
                  value={reverseConfig.remotePort}
                  onChange={(event) =>
                    setReverseConfig({
                      ...reverseConfig,
                      remotePort: Number(event.target.value),
                    })
                  }
                />
              </label>
              <label>
                Local target host
                <input
                  value={reverseConfig.localTargetHost}
                  onChange={(event) =>
                    setReverseConfig({
                      ...reverseConfig,
                      localTargetHost: event.target.value,
                    })
                  }
                />
              </label>
              <label>
                Local target port
                <input
                  type="number"
                  min="1"
                  max="65535"
                  value={reverseConfig.localTargetPort}
                  onChange={(event) =>
                    setReverseConfig({
                      ...reverseConfig,
                      localTargetPort: Number(event.target.value),
                    })
                  }
                />
              </label>
              <button type="button" onClick={startReverse}>
                Reverse 시작
              </button>
            </div>
          </>
        )}
        {error ? <div className="error-box">{error}</div> : null}
      </section>

      <section className="tunnel-list">
        <h3>실행 중인 터널</h3>
        {tunnelList.length === 0 ? (
          <div className="empty-state">실행 중인 터널이 없습니다.</div>
        ) : (
          tunnelList.map((tunnel) => (
            <article key={tunnel.tunnelId} className="tunnel-row">
              <div>
                <h4>{tunnel.name}</h4>
                <p>
                  {tunnel.direction === "reverse"
                    ? `${tunnel.remoteAddress}:${tunnel.remotePort} -> ${tunnel.targetHost}:${tunnel.targetPort}`
                    : `${tunnel.localAddress}:${tunnel.localPort} -> ${tunnel.targetHost}:${tunnel.targetPort}`}
                </p>
                {tunnel.message ? <p>{tunnel.message}</p> : null}
              </div>
              <span>{tunnel.status}</span>
              <strong>{tunnel.connectionCount}</strong>
              <button
                type="button"
                className="secondary-button"
                onClick={() => stop(tunnel.tunnelId)}
                disabled={tunnel.status === "stopped"}
              >
                중지
              </button>
            </article>
          ))
        )}
      </section>
    </div>
  );
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
