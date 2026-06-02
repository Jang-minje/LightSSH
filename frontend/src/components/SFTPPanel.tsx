import type { TransferProgressEvent } from "../api/sftp";

type SFTPPanelProps = {
  remotePath: string;
  transfers: Record<string, TransferProgressEvent>;
  onRemotePathChange: (path: string) => void;
};

export function SFTPPanel({
  remotePath,
  transfers,
  onRemotePathChange,
}: SFTPPanelProps) {
  const transferList = Object.values(transfers).sort((a, b) =>
    a.transferId.localeCompare(b.transferId),
  );

  return (
    <div className="sftp-simple-panel">
      <section className="sftp-simple-card">
        <h2>SFTP 업로드</h2>
        <label>
          원격 경로
          <input
            value={remotePath}
            onChange={(event) => onRemotePathChange(event.target.value)}
            placeholder="/mnt/dev/test"
          />
        </label>
      </section>
      <section className="transfer-list">
        <h3>전송</h3>
        {transferList.length === 0 ? (
          <div className="empty-state">진행 중인 전송이 없습니다.</div>
        ) : (
          transferList.map((transfer) => (
            <div key={transfer.transferId} className="transfer-row">
              <div>
                <strong>{transfer.direction}</strong>
                <span>{transfer.status}</span>
                <p>{transfer.remotePath}</p>
              </div>
              <progress
                value={transfer.total > 0 ? transfer.bytes : 0}
                max={transfer.total > 0 ? transfer.total : 1}
              />
            </div>
          ))
        )}
      </section>
    </div>
  );
}
