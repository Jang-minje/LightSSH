type HelpModalProps = {
  onClose: () => void;
};

export function HelpModal({ onClose }: HelpModalProps) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal help-modal" role="dialog" aria-modal="true">
        <div className="modal-title-row">
          <h2>도움말</h2>
          <button type="button" className="secondary-button" onClick={onClose}>
            닫기
          </button>
        </div>

        <section className="help-section">
          <h3>터미널</h3>
          <ul>
            <li>프로필 정보를 입력한 뒤 연결을 누르면 새 터미널 탭이 열립니다.</li>
            <li>저장된 프로필의 선택 버튼을 누르면 좌측 입력 영역에 불러옵니다.</li>
            <li>터미널에 로컬 파일을 드래그하면 현재 원격 경로로 업로드합니다.</li>
            <li>터미널에서 su로 계정이 바뀐 경우 파일 전송은 현재 터미널 계정 기준으로 시도합니다.</li>
          </ul>
        </section>

        <section className="help-section">
          <h3>터널</h3>
          <ul>
            <li>로컬 터널은 내 PC 포트를 SSH 서버를 통해 대상 서버로 연결합니다.</li>
            <li>리버스 터널은 SSH 서버 쪽 포트를 내 PC의 대상 포트로 연결합니다.</li>
            <li>리버스 터널의 바인드 주소는 서버의 GatewayPorts 설정에 따라 제한될 수 있습니다.</li>
          </ul>
        </section>

        <section className="help-section">
          <h3>단축키와 마우스</h3>
          <dl className="shortcut-list">
            <div>
              <dt>우클릭</dt>
              <dd>선택 영역이 있으면 복사, 없으면 붙여넣기</dd>
            </div>
            <div>
              <dt>Shift + 우클릭</dt>
              <dd>복사, 붙여넣기, 내 PC로 가져오기 메뉴 열기</dd>
            </div>
            <div>
              <dt>Ctrl + V</dt>
              <dd>클립보드 내용을 터미널에 붙여넣기</dd>
            </div>
            <div>
              <dt>Ctrl + Shift + C</dt>
              <dd>터미널 선택 영역 복사</dd>
            </div>
            <div>
              <dt>Ctrl + 마우스 휠</dt>
              <dd>터미널 위에서는 터미널 글꼴 크기, 좌우 패널 위에서는 앱 글꼴 크기 조절</dd>
            </div>
          </dl>
        </section>
      </section>
    </div>
  );
}
