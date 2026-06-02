import type { HostKeyPrompt } from "../api/ssh";

type HostKeyModalProps = {
  prompt: HostKeyPrompt | null;
  onDecision: (accept: boolean) => void | Promise<void>;
};

export function HostKeyModal({ prompt, onDecision }: HostKeyModalProps) {
  if (!prompt) {
    return null;
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal" role="dialog" aria-modal="true">
        <h2>
          {prompt.changed
            ? "SSH 호스트 키가 변경됨"
            : "알 수 없는 SSH 호스트 키"}
        </h2>
        {prompt.changed ? (
          <p>
            저장된 host key와 서버가 보낸 key가 다릅니다. 서버 재설치,
            장비 교체, IP 재사용이 확인된 경우에만 교체하세요. 확인되지 않은
            변경은 중간자 공격 가능성이 있습니다.
          </p>
        ) : (
          <p>
            이 서버는 아직 known_hosts에 등록되어 있지 않습니다. 서버
            fingerprint가 맞는지 확인한 뒤 신뢰할 경우에만 승인하세요.
          </p>
        )}
        <dl className="hostkey-details">
          <div>
            <dt>Host</dt>
            <dd>{prompt.host}</dd>
          </div>
          <div>
            <dt>Address</dt>
            <dd>{prompt.address}</dd>
          </div>
          <div>
            <dt>Key type</dt>
            <dd>{prompt.keyType}</dd>
          </div>
          <div>
            <dt>Fingerprint</dt>
            <dd>{prompt.fingerprint}</dd>
          </div>
          <div>
            <dt>known_hosts</dt>
            <dd>{prompt.knownHosts}</dd>
          </div>
        </dl>
        <div className="modal-actions">
          <button
            type="button"
            className={prompt.changed ? "danger-button" : undefined}
            onClick={() => onDecision(true)}
          >
            {prompt.changed ? "기존 키 교체" : "신뢰하고 저장"}
          </button>
          <button
            type="button"
            className="secondary-button"
            onClick={() => onDecision(false)}
          >
            취소
          </button>
        </div>
      </section>
    </div>
  );
}
