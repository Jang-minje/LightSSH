import type { RemoteEntry } from "../api/sftp";

type RemoteDownloadModalProps = {
  entries: RemoteEntry[];
  localDirectory: string;
  loading: boolean;
  overwrite: boolean;
  remotePath: string;
  selectedPaths: Set<string>;
  onClose: () => void;
  onDownload: () => void;
  onBrowseLocalDirectory: () => void;
  onLocalDirectoryChange: (path: string) => void;
  onOverwriteChange: (overwrite: boolean) => void;
  onRefresh: () => void;
  onSelect: (path: string, selected: boolean) => void;
};

export function RemoteDownloadModal({
  entries,
  localDirectory,
  loading,
  overwrite,
  remotePath,
  selectedPaths,
  onClose,
  onDownload,
  onBrowseLocalDirectory,
  onLocalDirectoryChange,
  onOverwriteChange,
  onRefresh,
  onSelect,
}: RemoteDownloadModalProps) {
  const fileEntries = entries.filter((entry) => !entry.isDir);

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal remote-download-modal" role="dialog" aria-modal="true">
        <div className="modal-title-row">
          <h2>내 PC로 가져오기</h2>
          <button type="button" className="secondary-button" onClick={onClose}>
            닫기
          </button>
        </div>
        <label className="modal-field">
          원격 경로
          <input value={remotePath} readOnly />
        </label>
        <label className="modal-field">
          저장 위치
          <input
            value={localDirectory}
            onChange={(event) => onLocalDirectoryChange(event.target.value)}
            onClick={onBrowseLocalDirectory}
            placeholder="D:\\Downloads"
            readOnly
          />
        </label>
        <label className="checkbox-row">
          <input
            type="checkbox"
            checked={overwrite}
            onChange={(event) => onOverwriteChange(event.target.checked)}
          />
          같은 이름 덮어쓰기
        </label>
        <div className="remote-browser">
          {loading ? (
            <div className="empty-state">목록을 불러오는 중입니다.</div>
          ) : fileEntries.length === 0 ? (
            <div className="empty-state">다운로드할 파일이 없습니다.</div>
          ) : (
            fileEntries.map((entry) => (
              <div key={entry.path} className="remote-entry-row">
                <label className="remote-entry-check">
                  <input
                    type="checkbox"
                    checked={selectedPaths.has(entry.path)}
                    onChange={(event) =>
                      onSelect(entry.path, event.target.checked)
                    }
                  />
                  <span>{entry.name}</span>
                </label>
                <span>{formatBytes(entry.size)}</span>
              </div>
            ))
          )}
        </div>
        <div className="modal-actions">
          <button type="button" className="secondary-button" onClick={onRefresh}>
            새로고침
          </button>
          <button
            type="button"
            onClick={onDownload}
            disabled={selectedPaths.size === 0 || loading}
          >
            선택 다운로드
          </button>
        </div>
      </section>
    </div>
  );
}

function formatBytes(size: number): string {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}
