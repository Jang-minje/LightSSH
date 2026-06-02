type UploadConfirmModalProps = {
  localPath: string | null;
  remotePath: string;
  transferMode: "sftp" | "terminal";
  onRemotePathChange: (remotePath: string) => void;
  onTransferModeChange: (transferMode: "sftp" | "terminal") => void;
  onOverwrite: () => void;
  onRename: () => void;
  onCancel: () => void;
};

export function UploadConfirmModal({
  localPath,
  remotePath,
  transferMode,
  onRemotePathChange,
  onTransferModeChange,
  onOverwrite,
  onRename,
  onCancel,
}: UploadConfirmModalProps) {
  if (!localPath) {
    return null;
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal upload-modal" role="dialog" aria-modal="true">
        <h2>SFTP 업로드</h2>
        <label className="modal-field">
          원격경로
          <input
            value={remotePath}
            onChange={(event) => onRemotePathChange(event.target.value)}
          />
        </label>
        <dl className="hostkey-details">
          <div>
            <dt>대상파일</dt>
            <dd>{localPath}</dd>
          </div>
        </dl>
        <fieldset className="transfer-mode-options">
          <legend>전송 방식</legend>
          <label>
            <input
              type="radio"
              name="upload-transfer-mode"
              checked={transferMode === "sftp"}
              onChange={() => onTransferModeChange("sftp")}
            />
            <span>
              빠른 SFTP
              <small>프로필 로그인 계정 권한으로 업로드합니다.</small>
            </span>
          </label>
          <label>
            <input
              type="radio"
              name="upload-transfer-mode"
              checked={transferMode === "terminal"}
              onChange={() => onTransferModeChange("terminal")}
            />
            <span>
              현재 터미널 계정
              <small>su 이후 계정 권한을 쓰지만 속도가 느립니다.</small>
            </span>
          </label>
        </fieldset>
        <div className="modal-actions">
          <button type="button" onClick={onOverwrite}>
            덮어쓰기
          </button>
          <button type="button" onClick={onRename}>
            이름바꿔넣기
          </button>
          <button type="button" className="secondary-button" onClick={onCancel}>
            취소
          </button>
        </div>
      </section>
    </div>
  );
}
