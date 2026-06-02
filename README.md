# LightSSH

LightSSH는 회사 내부 유지보수용 경량 SSH 데스크톱 클라이언트입니다. 현재 단계에서는 Wails 기반 앱 골격, React 프론트엔드, 설정 파일 로딩/저장 모듈, connection profile 타입과 SSH 터미널 MVP를 포함합니다. SFTP와 터널링은 아직 구현하지 않았습니다.

## 요구 환경

- Go 1.23 이상
- Node.js 20 이상
- Wails CLI v2

Wails CLI 설치:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 실행

```powershell
npm install --prefix frontend
wails dev
```

## 빌드

```powershell
wails build
```

## 테스트

```powershell
gofmt -w .
go test ./...
npm run build --prefix frontend
```

## 현재 포함된 모듈

- `backend/app`: Wails 바인딩 앱 서비스
- `backend/config`: connection profile 타입, 설정 파일 로딩/저장, validation
- `backend/sshclient`: SSH 접속, PTY 요청, shell 실행, stdin/stdout/stderr relay, resize 처리
- `backend/sftpclient`: 향후 SFTP 구현 위치
- `backend/tunnel`: 향후 로컬/리버스 터널링 구현 위치
- `backend/security`: 향후 known_hosts와 credential 처리 구현 위치
- `frontend/src/components`: React 컴포넌트
- `frontend/src/pages`: 화면 단위 컴포넌트
- `frontend/src/api`: 프론트엔드 API 타입과 호출 모듈

## SSH 터미널 MVP

- SSH 구현은 `golang.org/x/crypto/ssh`를 사용합니다.
- 터미널 UI는 xterm.js 기반의 `@xterm/xterm`을 사용합니다.
- 비밀번호와 개인키 passphrase는 접속 요청에서만 사용하며 설정 파일에 저장하지 않습니다.
- host key 검증은 사용자 홈의 `.ssh/known_hosts` 파일을 사용합니다.
- `known_hosts`가 없거나 대상 서버 키가 등록되어 있지 않으면 연결이 실패합니다.
- 연결 종료, 인증 실패, 타임아웃, host key 검증 실패는 UI에 한글 메시지로 표시됩니다.
