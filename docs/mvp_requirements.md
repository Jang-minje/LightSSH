# 내부 유지보수용 경량 SSH 한글 클라이언트 MVP 요구사항

## 1. MVP 요구사항 문서

### 제품 목적

회사 내부 운영자와 유지보수 담당자가 Windows 데스크톱 환경에서 SSH 터미널 접속, SFTP 파일 송수신, 로컬/리버스 터널링을 안전하게 수행할 수 있는 경량 클라이언트를 제공한다.

이 제품은 PuTTY처럼 SSH 터미널 접속을 제공하되, MVP에서는 내부 유지보수 업무에 필요한 핵심 기능만 포함한다. FTP, Telnet, Serial, RDP, SCP, X11 forwarding, 복잡한 터미널 세부 옵션은 제공하지 않는다.

### 대상 사용자

- 회사 내부 서버 유지보수 담당자
- 운영 서버에 SSH로 접속해야 하는 개발자 또는 운영자
- SFTP로 설정 파일, 로그, 배포 산출물을 업로드/다운로드해야 하는 사용자
- 로컬 포트 포워딩 또는 리버스 포트 포워딩으로 내부 서비스를 점검해야 하는 사용자

### MVP 핵심 사용자 시나리오

1. 사용자는 접속 프로필을 생성한다.
2. 사용자는 호스트, 포트, 사용자명, 인증 방식을 입력한다.
3. 앱은 `known_hosts` 기반으로 호스트 키를 검증한다.
4. 사용자는 SSH 터미널 세션을 시작한다.
5. 사용자는 터미널에서 명령을 실행하고 결과를 확인한다.
6. 사용자는 SFTP 화면에서 파일을 드래그앤드롭하여 업로드하거나 다운로드한다.
7. 사용자는 로컬 터널링 또는 리버스 터널링 규칙을 등록하고 시작/중지한다.
8. 사용자는 연결 실패, 인증 실패, 타임아웃, 끊김 상태를 명확한 한글 메시지로 확인한다.

### MVP 성공 기준

- SSH 터미널 접속이 안정적으로 동작한다.
- `known_hosts` 검증을 우회하지 않는다.
- 비밀번호, 개인키 passphrase, 세션 secret이 평문 저장되거나 로그에 남지 않는다.
- SFTP 업로드/다운로드가 드래그앤드롭 UI로 동작한다.
- 로컬 터널링과 리버스 터널링을 시작/중지하고 상태를 확인할 수 있다.
- 실패, 끊김, 타임아웃이 사용자에게 한글 메시지로 표시된다.
- 기능별 구현이 작은 PR 단위로 분리 가능하다.

## 2. 기능 범위와 제외 범위

### 포함 범위

- SSH 터미널 접속
- 접속 프로필 저장
- 비밀번호 인증
- 개인키 인증
- ssh-agent 인증
- `known_hosts` 기반 host key 검증
- 알 수 없는 host key 경고 및 사용자 승인 흐름
- 변경된 host key 차단 및 명시적 경고
- xterm.js 기반 터미널 UI
- 터미널 resize 전달
- SFTP 파일 목록 조회
- SFTP 파일 업로드
- SFTP 파일 다운로드
- 드래그앤드롭 기반 업로드/다운로드 UI
- 로컬 포트 포워딩
- 리버스 포트 포워딩
- 연결 로그 표시
- 터널 상태 표시
- 연결 실패, 인증 실패, 네트워크 타임아웃, 세션 끊김 처리

### 제외 범위

- FTP
- Telnet
- Serial
- RDP
- SCP
- X11 forwarding
- PuTTY 소스 코드 복사 또는 포팅
- 자체 암호화 알고리즘 구현
- 복잡한 터미널 옵션 UI
- 세션 매크로
- 터미널 테마 마켓
- 멀티 프로토콜 접속 관리자
- 서버 관리 자동화 스크립트 실행기
- 팀 단위 프로필 동기화
- 중앙 정책 서버 연동

## 3. 프로젝트 폴더 구조

```text
custom_putty/
  agents.md
  README.md
  docs/
    mvp_requirements.md
    architecture.md
    security.md
    testing.md
  app/
    main.go
    wails.json
    go.mod
    go.sum
    internal/
      app/
      sshclient/
      sftpclient/
      tunnel/
      profile/
      knownhosts/
      credentials/
      logging/
      errors/
    pkg/
      contracts/
  frontend/
    package.json
    tsconfig.json
    vite.config.ts
    src/
      main.tsx
      App.tsx
      bindings/
      components/
      screens/
      stores/
      styles/
      utils/
  test/
    fixtures/
    integration/
```

### 폴더 역할

- `docs/`: 요구사항, 설계, 보안, 테스트 문서
- `app/`: Wails Go 백엔드
- `app/internal/`: 외부 공개 대상이 아닌 Go 내부 모듈
- `app/pkg/contracts/`: 프론트엔드와 공유할 DTO, 이벤트 계약
- `frontend/`: Wails 프론트엔드
- `test/`: 통합 테스트 fixture와 테스트 서버 구성

## 4. 주요 Go 패키지 구조

### `internal/app`

- Wails 앱 생명주기 관리
- 프론트엔드에서 호출하는 public method 제공
- SSH, SFTP, 터널 모듈 조합
- 앱 시작/종료 시 세션 정리

### `internal/profile`

- 접속 프로필 모델
- 프로필 생성/수정/삭제/조회
- 프로필 유효성 검증
- 저장 가능한 필드 제한
- 기본 저장 필드: host, port, username, auth type, key path, metadata
- 비밀번호와 passphrase는 기본 저장 금지

### `internal/credentials`

- 인증 입력값의 수명 관리
- 메모리 내 일시 보관
- 향후 OS credential storage 연동 지점
- 로그 마스킹 정책 제공

### `internal/knownhosts`

- `known_hosts` 파일 로드
- host key callback 구성
- 알 수 없는 host key 처리
- 변경된 host key 탐지
- 사용자 승인 후 `known_hosts` 갱신

### `internal/sshclient`

- `golang.org/x/crypto/ssh` 기반 SSH 접속
- 인증 방식 구성
- SSH session 생성
- PTY 요청
- terminal resize 처리
- stdin/stdout/stderr stream 연결
- reconnect 판단에 필요한 오류 분류

### `internal/sftpclient`

- `github.com/pkg/sftp/v2` 기반 SFTP client 생성
- 원격 디렉터리 목록 조회
- 업로드
- 다운로드
- mkdir, rename, delete 등 MVP에 필요한 최소 파일 작업
- 경로 정규화와 traversal 방어

### `internal/tunnel`

- 로컬 포트 포워딩
- 리버스 포트 포워딩
- 터널 시작/중지
- 포트 바인딩 실패 처리
- 연결별 goroutine 수명 관리
- 터널 상태 이벤트 발행

### `internal/logging`

- 연결 로그 이벤트 구조화
- 민감정보 마스킹
- 프론트엔드 표시용 한글 메시지 매핑
- debug 로그와 사용자 표시 로그 분리

### `internal/errors`

- 인증 실패
- host key 실패
- 네트워크 실패
- 타임아웃
- 사용자 취소
- 세션 종료
- SFTP 전송 실패
- 터널 바인딩 실패

### `pkg/contracts`

- Wails 바인딩용 request/response DTO
- 프론트엔드 이벤트 타입
- SSH session 상태 enum
- SFTP transfer 상태 enum
- Tunnel 상태 enum

## 5. 프론트엔드 화면 구성

### 전체 레이아웃

- 좌측: 접속 프로필 목록
- 중앙: 현재 SSH 터미널 또는 SFTP 화면
- 우측 또는 하단: 연결 로그, 터널 상태, 전송 상태
- 상단: 현재 접속 상태, 접속/해제 버튼, 설정 버튼

### 화면 목록

#### 접속 프로필 화면

- 프로필 목록
- 프로필 생성 버튼
- 프로필 수정 버튼
- 프로필 삭제 버튼
- 최근 접속 표시
- 접속 상태 표시

#### 프로필 편집 화면

- 프로필 이름
- 호스트
- 포트
- 사용자명
- 인증 방식 선택
- 개인키 경로 선택
- ssh-agent 사용 여부
- 접속 타임아웃
- 저장 버튼
- 테스트 접속 버튼

비밀번호와 passphrase는 저장 기본값에서 제외하며, 접속 시 입력받는다.

#### SSH 터미널 화면

- xterm.js 터미널
- 연결 상태 표시
- 재접속 버튼
- 세션 종료 버튼
- 터미널 크기 변경 자동 반영
- 복사/붙여넣기 기본 동작

#### SFTP 화면

- 로컬 파일 드롭 영역
- 원격 디렉터리 파일 목록
- 업로드 진행률
- 다운로드 진행률
- 실패한 전송 재시도
- 전송 취소
- 경로 이동
- 새로고침

#### 터널링 화면

- 로컬 터널 목록
- 리버스 터널 목록
- 터널 생성/수정/삭제
- 터널 시작/중지
- 로컬 주소, 로컬 포트, 원격 주소, 원격 포트 입력
- 바인딩 상태
- 마지막 오류 메시지

#### 로그 화면

- 연결 시도
- 인증 결과
- host key 검증 결과
- SFTP 전송 결과
- 터널 시작/중지
- 오류와 타임아웃
- 민감정보 마스킹 적용

## 6. SSH/SFTP/터널링 모듈 설계

### SSH 연결 흐름

1. 프론트엔드가 프로필 ID와 인증 입력값으로 접속 요청
2. `profile` 모듈이 프로필 로드 및 검증
3. `credentials` 모듈이 인증 입력값을 일시 객체로 구성
4. `knownhosts` 모듈이 host key callback 생성
5. `sshclient` 모듈이 `ssh.ClientConfig` 구성
6. `ssh.Dial` 실행
7. 성공 시 session registry에 client 저장
8. 터미널 요청 시 PTY session 생성
9. stdin/stdout/stderr stream을 Wails event와 연결
10. 세션 종료 또는 앱 종료 시 client/session 정리

### 인증 방식

- 비밀번호 인증: 접속 시 입력받고 저장하지 않는다.
- 개인키 인증: key path만 프로필에 저장한다.
- 개인키 passphrase: 접속 시 입력받고 저장하지 않는다.
- ssh-agent 인증: OS agent socket 또는 Windows agent 연동 가능 범위 내에서 사용한다.

### Host Key 검증

- 기본 정책은 `known_hosts` 검증 필수이다.
- 알 수 없는 host key는 사용자에게 fingerprint와 경고를 표시한다.
- 사용자가 명시적으로 승인한 경우에만 `known_hosts`에 추가한다.
- 기존 host key와 다른 경우 기본적으로 접속을 차단한다.
- 변경된 host key를 자동으로 덮어쓰지 않는다.

### SFTP 설계

1. 기존 SSH client에서 SFTP client 생성
2. 원격 경로 목록 조회
3. 업로드 요청 시 로컬 경로 검증
4. 원격 경로 정규화
5. 파일 stream 전송
6. 진행률 이벤트 발행
7. 완료/실패/취소 상태 이벤트 발행

대용량 파일은 전체 파일을 메모리에 올리지 않고 stream 기반으로 처리한다.

### 로컬 터널링 설계

로컬 터널링은 로컬 포트를 listen하고, 연결이 들어올 때마다 SSH server를 통해 원격 대상 주소로 `Dial`한다.

```text
local app -> local listen port -> SSH client -> remote target host:port
```

필수 처리:

- 로컬 포트 바인딩 실패
- 중복 포트 사용
- 터널별 연결 수 관리
- 터널 중지 시 listener close
- 활성 연결 graceful close
- 상태 이벤트 발행

### 리버스 터널링 설계

리버스 터널링은 SSH server 측에서 원격 포트를 listen하고, 연결이 들어오면 로컬 대상 주소로 전달한다.

```text
remote listen port -> SSH client -> local target host:port
```

필수 처리:

- 원격 포트 바인딩 실패
- 서버 정책상 tcpip-forward 거부
- 연결 accept loop 종료
- 터널 중지 시 remote listener close
- 상태 이벤트 발행

### 세션과 리소스 관리

- SSH client는 session ID로 registry에서 관리한다.
- 터미널 session, SFTP client, tunnel은 SSH client에 종속된다.
- 부모 SSH client 종료 시 하위 리소스를 모두 종료한다.
- 모든 장기 작업은 context cancellation을 지원한다.
- 타임아웃은 접속, 인증, SFTP 전송, 터널 연결마다 분리한다.

## 7. 보안 요구사항

### 암호화와 라이브러리

- 자체 암호화 구현을 금지한다.
- SSH는 `golang.org/x/crypto/ssh`를 사용한다.
- SFTP는 `github.com/pkg/sftp/v2`를 사용한다.
- 비밀번호 저장, 개인키 passphrase 저장, 임의 암호화 저장소 구현을 MVP에서 금지한다.

### 민감정보 저장 정책

- 기본 저장 허용: profile name, host, port, username, auth type, key path, metadata
- 기본 저장 금지: password, private key content, private key passphrase, session token, agent secret
- OS credential storage 연동은 별도 credential module 구현 후에만 허용한다.

### Host Key 정책

- `known_hosts` 검증은 필수이다.
- host key 검증 비활성화 옵션은 MVP에서 제공하지 않는다.
- unknown key는 사용자 승인 전 접속하지 않는다.
- changed key는 차단하고, 수동 확인 절차를 요구한다.
- fingerprint는 SHA256 기반으로 표시한다.

### 로그 정책

- 비밀번호, passphrase, private key, raw auth payload를 로그에 남기지 않는다.
- 오류 메시지에 민감정보가 포함될 수 있는 경우 마스킹한다.
- 사용자 표시 로그와 debug 로그를 분리한다.
- debug 로그도 기본적으로 민감정보를 포함하지 않는다.

### 파일 전송 보안

- 로컬 파일 경로와 원격 파일 경로를 검증한다.
- 원격 경로 traversal을 방지한다.
- 다운로드 시 덮어쓰기 확인을 제공한다.
- 업로드 실패 시 부분 파일 처리 정책을 명확히 한다.
- 심볼릭 링크 처리는 MVP에서 보수적으로 제한하거나 명시적으로 표시한다.

### 터널링 보안

- 바인딩 주소 기본값은 `127.0.0.1`로 둔다.
- `0.0.0.0` 바인딩은 명시적 경고 후 허용한다.
- 리버스 터널 원격 bind 주소는 서버 정책과 사용자 입력을 분리해 표시한다.
- 터널 상태와 대상 주소를 UI에 명확히 표시한다.

### 장애 처리

- 접속 실패
- 인증 실패
- host key 불일치
- 네트워크 타임아웃
- 세션 끊김
- SFTP 전송 실패
- 터널 바인딩 실패
- 사용자가 취소한 작업

모든 실패는 사용자에게 한글 메시지, 기술 코드, 재시도 가능 여부로 표시한다.

## 8. 구현 순서

각 단계는 작은 PR 단위로 구현 가능해야 한다.

### PR 1. 프로젝트 스캐폴딩

- Wails 앱 생성
- Go module 구성
- 프론트엔드 TypeScript 프로젝트 구성
- 기본 빌드/실행 확인

### PR 2. 공통 계약과 오류 모델

- Wails request/response DTO 정의
- 상태 enum 정의
- 공통 오류 타입 정의
- 한글 오류 메시지 매핑 초안 작성

### PR 3. 프로필 저장과 검증

- 프로필 모델 구현
- 프로필 CRUD
- 저장 가능 필드 제한
- host, port, username, auth type validation

### PR 4. known_hosts 모듈

- known_hosts 로드
- host key callback 구성
- unknown key 감지
- changed key 감지
- 승인 후 known_hosts 추가 흐름

### PR 5. SSH 접속 모듈

- 비밀번호 인증
- 개인키 인증
- ssh-agent 인증
- 접속 타임아웃
- 연결 상태 이벤트

### PR 6. 터미널 세션

- xterm.js 화면
- PTY session 생성
- 입출력 stream 연결
- resize 처리
- 세션 종료 처리

### PR 7. SFTP 기본 기능

- SFTP client 생성
- 원격 디렉터리 목록
- 경로 이동
- 파일 다운로드
- 파일 업로드

### PR 8. SFTP 드래그앤드롭과 전송 상태

- 드래그앤드롭 업로드 UI
- 다운로드 대상 선택
- 진행률 표시
- 취소와 재시도
- 전송 실패 메시지

### PR 9. 로컬 터널링

- 로컬 터널 설정 모델
- 로컬 listener 생성
- SSH direct-tcpip 연결
- 시작/중지
- 상태와 오류 표시

### PR 10. 리버스 터널링

- 리버스 터널 설정 모델
- remote listener 생성
- local target 연결
- 시작/중지
- 상태와 오류 표시

### PR 11. 로그와 민감정보 마스킹

- 연결 로그
- SFTP 로그
- 터널 로그
- 마스킹 필터
- 프론트엔드 로그 화면

### PR 12. 안정화와 패키징

- 앱 종료 시 리소스 정리
- 타임아웃 정책 정리
- 통합 테스트 보강
- Windows 빌드 패키징
- 사용자 문서 작성

## 9. 각 단계별 테스트 방법

### PR 1. 프로젝트 스캐폴딩 테스트

- `go test ./...`
- 프론트엔드 typecheck
- Wails dev 실행
- Windows에서 앱 창 표시 확인

### PR 2. 공통 계약과 오류 모델 테스트

- DTO serialization 단위 테스트
- 상태 enum 매핑 테스트
- 오류 코드별 한글 메시지 테스트
- 민감정보가 오류 문자열에 포함되지 않는지 테스트

### PR 3. 프로필 저장과 검증 테스트

- 정상 프로필 저장/조회/수정/삭제 테스트
- host 누락 실패
- port 범위 초과 실패
- username 누락 실패
- password 저장 시도 거부 테스트
- key path 저장 허용 테스트

### PR 4. known_hosts 모듈 테스트

- known host 접속 허용 테스트
- unknown host 감지 테스트
- unknown host 사용자 승인 후 추가 테스트
- changed host key 차단 테스트
- 잘못된 known_hosts 파일 파싱 실패 처리 테스트

### PR 5. SSH 접속 모듈 테스트

- 테스트 SSH 서버 기반 비밀번호 인증 성공
- 비밀번호 인증 실패
- 개인키 인증 성공
- passphrase 필요 키 처리
- ssh-agent 인증 stub 테스트
- 접속 타임아웃 테스트
- 네트워크 오류 분류 테스트

### PR 6. 터미널 세션 테스트

- PTY 요청 성공
- stdin 입력 전달
- stdout 출력 수신
- terminal resize 전달
- 세션 종료 이벤트 발생
- 서버가 연결을 끊었을 때 UI 상태 변경

### PR 7. SFTP 기본 기능 테스트

- 원격 디렉터리 목록 조회
- 파일 업로드 성공
- 파일 다운로드 성공
- 없는 경로 접근 실패
- 권한 없는 경로 접근 실패
- 대용량 파일 stream 처리 확인

### PR 8. SFTP 드래그앤드롭과 전송 상태 테스트

- 드래그앤드롭 업로드 트리거
- 진행률 이벤트 표시
- 업로드 취소
- 다운로드 취소
- 실패 후 재시도
- 같은 이름 파일 덮어쓰기 확인

### PR 9. 로컬 터널링 테스트

- 로컬 포트 listen 성공
- 포트 중복 시 실패
- 로컬 터널을 통한 원격 서비스 접근
- SSH 연결 끊김 시 터널 종료
- 터널 중지 시 listener close
- 상태 이벤트 표시

### PR 10. 리버스 터널링 테스트

- remote listener 생성 성공
- 서버가 리버스 포워딩을 거부할 때 실패 처리
- remote port 중복 실패 처리
- 리버스 터널을 통한 로컬 서비스 접근
- 터널 중지 시 remote listener close
- 상태 이벤트 표시

### PR 11. 로그와 민감정보 마스킹 테스트

- password 문자열 마스킹
- passphrase 문자열 마스킹
- private key content 로그 누락 확인
- 연결 로그 표시
- SFTP 로그 표시
- 터널 로그 표시

### PR 12. 안정화와 패키징 테스트

- 앱 종료 시 SSH session 정리
- 앱 종료 시 SFTP transfer 취소
- 앱 종료 시 tunnel listener close
- Windows 패키지 실행 테스트
- 네트워크 단절 시 UI 상태 테스트
- 장시간 세션 유지 테스트
- 최소 수동 회귀 테스트 체크리스트 수행

### 공통 테스트 원칙

- config, profile, validation, path handling은 unit test로 검증한다.
- SSH/SFTP/터널은 테스트 서버 기반 integration test stub을 둔다.
- 실제 사내 서버 의존 테스트는 기본 CI에서 제외하고 수동 테스트로 분리한다.
- 모든 PR은 formatting, linting, unit test를 통과해야 한다.
- 보안 관련 테스트는 회귀 방지를 위해 별도 test case로 유지한다.
