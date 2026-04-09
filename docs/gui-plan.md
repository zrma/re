# GUI 도입 계획

## 요약

`re`의 첫 GUI 목표는 "보기 좋은 앱"보다 "기존 rename 동작을 더 안전하고 편하게 쓰는 것"이다.

- 핵심 rename 로직은 계속 `pkg/re`가 소유한다.
- GUI는 plan을 보여주고, 사용자가 명시적으로 적용하도록 돕는 얇은 껍데기로 유지한다.
- preview와 apply는 반드시 같은 plan을 공유해야 한다.
- 첫 단계에서는 기능 확장보다 안정성 유지와 사용성 개선을 우선한다.

현재 우선순위 기준으로는 `Wails`보다 `Fyne`가 더 적합하다. 이유는 Go 코드 한 스택 안에서 GUI를 붙일 수 있고, 대량 rename 도구에 중요한 안전 경계를 단순하게 유지하기 쉽기 때문이다.

## 상태 표기

- `[x]`: 완료
- `[ ]`: 미완료

## 현재 진행 상태

- [x] 1차 GUI 방향을 `Fyne` 기반으로 정리했다.
- [x] GUI의 역할을 "rename 엔진 대체"가 아니라 "plan/apply를 감싼 얇은 화면"으로 정리했다.
- [x] 1차 범위와 비목표를 문서화했다.
- [x] `pkg/re` 공용 서비스 계층을 분리했다.
- [x] CLI를 공용 서비스 위로 재배선했다.
- [x] preview 직전 상태를 스냅샷으로 잡고, apply 전에 재검증하는 경로를 추가했다.
- [x] service layer 전용 테스트를 추가해 rule/ai/noop/needs_review/apply/stale preview 경로를 고정했다.
- [x] GUI 상태 회귀 테스트를 추가해 preview refresh, stale apply rejection, empty state, stale error clearing 경로를 고정했다.
- [x] GUI 스켈레톤을 만들었다.
- [x] 폴더 선택, preview, skip 목록, apply 확인, stale preview 메시지까지 기본 UX를 연결했다.
- [x] 경로 직접 입력, warning/error 배너, 선택 상세 패널, 위험도 우선 정렬까지 1차 안전 UX 보강을 반영했다.
- [x] 경로나 AI 옵션이 바뀌면 이전 preview 적용을 막고 재생성을 요구하도록 보강했다.
- [x] 과한 카드 사용을 줄이고, 섹션 레이아웃과 요약 지표로 화면 구조를 정리했다.
- [x] warning은 배너 요약과 상세 다이얼로그로 나눠 과도하게 화면을 밀어내지 않게 했다.
- [ ] GUI 검증 체크리스트를 실제 구현 기준으로 소진하지 않았다.

## 목표

- 기존 CLI 동작과 rename 결과를 유지한다.
- GUI에서도 rename 전에 preview를 먼저 보여준다.
- GUI에서도 apply는 명시적 확인 이후에만 수행한다.
- 충돌, skip 이유, unresolved 상태를 사용자에게 분명하게 보여준다.
- CLI와 GUI가 동일한 서비스 계층을 재사용하도록 정리한다.

## 비목표

- 첫 단계에서 커스텀 rename 규칙 편집 UI를 넣지 않는다.
- GUI에서 destination 이름을 수동으로 수정하게 하지 않는다.
- GUI가 rename 규칙이나 파일 시스템 조작을 자체 구현하지 않는다.
- 첫 단계에서 웹 프론트엔드나 리치 데스크톱 UI를 목표로 하지 않는다.
- 하위 디렉터리 재귀 처리, 백업 관리, 히스토리 복원 같은 별도 기능을 함께 풀지 않는다.

## 왜 Fyne인가

현재 프로젝트는 핵심 로직이 이미 Go에 잘 모여 있다.

- 스캔: `pkg/re/scan.go`
- rule 해석: `pkg/re/resolve.go`
- plan 생성과 안전 게이트: `pkg/re/plan.go`
- 보고서 생성: `pkg/re/report.go`
- 실행 오케스트레이션: `pkg/re/re.go`

이 상태에서 GUI를 가장 안전하게 붙이려면:

- 같은 프로세스 안에서 `pkg/re`를 직접 호출하고
- CLI와 GUI가 같은 타입과 같은 plan을 재사용하고
- JS 프론트엔드, 상태 브리지, 별도 빌드 파이프라인을 추가하지 않는 편이 유리하다.

`Wails`는 더 풍부한 UI를 만들기 쉽지만, 현재 우선순위인 안정성 최우선과 최소 변경 기준에서는 `Fyne`가 더 잘 맞는다.

## 핵심 원칙

### 1. GUI는 `pkg/re`를 감싼다

GUI는 rename 규칙을 직접 구현하지 않는다. 스캔, 해석, plan 생성, 충돌 검증, rename 적용은 계속 `pkg/re`가 담당한다.

### 2. preview와 apply는 같은 plan을 사용한다

GUI가 preview 화면용 데이터를 따로 계산하고, apply 때 다른 경로를 타면 안 된다. 사용자가 본 plan과 실제 적용 plan이 달라지지 않게 해야 한다.

### 3. apply 직전 재검증을 둔다

preview를 띄운 뒤 사용자가 폴더를 건드렸을 수 있다. apply 직전에는 source/destination 상태가 preview 시점과 같은지 다시 확인하고, 달라졌으면 적용을 거부하고 새로고침을 요구하는 편이 안전하다.

### 4. GUI는 읽기 쉬운 상태를 보여주는 데 집중한다

첫 단계 GUI의 가치는 "복잡한 설정"이 아니라 "무엇이 rename되고, 무엇이 skip되며, 왜 그런지 한눈에 보는 것"이다.

## 제안 아키텍처

### 서비스 계층 분리

현재 `pkg/re/re.go`는 CLI 입출력까지 포함한 실행 오케스트레이션이다. GUI 도입 전, 아래처럼 공용 서비스 경계를 분리하는 편이 좋다.

```go
type PreviewRequest struct {
    TargetPath string
    Options    RunOptions
}

type PreviewResult struct {
    TargetPath  string
    ScanResult  ScanResult
    Resolution  ResolutionResult
    Plan        RenamePlan
    Report      RunReport
    Snapshot    DirectorySnapshot
}

func BuildPreview(ctx context.Context, req PreviewRequest) (PreviewResult, error)
func ApplyPreview(preview PreviewResult) (RunReport, error)
```

핵심은 CLI와 GUI가 둘 다 이 서비스 계층만 호출하게 만드는 것이다.

### 디렉터리 스냅샷

GUI에서는 preview와 apply 사이에 시간이 뜰 수 있으므로, apply 직전 재검증을 위한 스냅샷이 필요하다.

예시:

```go
type FileSnapshot struct {
    Path    string
    Size    int64
    ModTime time.Time
}

type DirectorySnapshot struct {
    Files []FileSnapshot
}
```

이 스냅샷은 강한 보안 검증이 목적이 아니라, preview 이후 디렉터리 상태가 바뀌었는지 감지하는 안전 장치다.

### CLI 정리 방향

CLI는 앞으로 아래 역할만 맡는다.

- flag 파싱
- `BuildPreview()` 호출
- text/json 출력
- 사용자가 `y`를 입력했을 때 `ApplyPreview()` 호출

즉, 대화형 확인만 CLI 고유 역할로 남기고 나머지는 공용 서비스로 내린다.

### GUI 구조

권장 배치는 다음과 같다.

- `cmd/re/main.go`: 기존 CLI 유지
- `cmd/re-gui/main.go`: GUI 진입점
- `internal/gui/...`: Fyne 화면과 상태 관리
- `pkg/re/...`: 공용 rename 서비스와 도메인 로직

이 구조면 나중에 GUI 프레임워크를 바꿔도 `pkg/re`는 그대로 재사용할 수 있다.

## 1단계 GUI 범위

첫 버전은 아래만 넣는다.

- 대상 폴더 선택
- AI fallback on/off
- 미리보기 테이블
- skip 목록
- 요약 통계
- 새로고침 버튼
- 적용 버튼
- 적용 전 최종 확인 다이얼로그

preview 테이블은 최소한 아래 열이 있으면 된다.

- source
- destination
- episode
- extension
- match source
- confidence

skip 목록은 아래 정보가 보이면 충분하다.

- source
- reason

## 1단계에서 일부러 넣지 않을 것

- destination 수동 편집
- 행 단위 체크 해제/개별 제외
- drag and drop 기반 규칙 작성
- 다중 폴더 탭
- 백업/undo UI
- 로그 뷰어

이런 기능은 편해 보이지만, 첫 버전부터 넣으면 plan과 apply 경계가 흐려지고 테스트 조합이 급격히 늘어난다.

## 사용자 흐름

### 1. 폴더 선택

사용자가 대상 폴더를 고른다.

### 2. preview 생성

GUI는 `BuildPreview()`를 호출한다.

- 스캔
- rule 기반 해석
- AI fallback 적용 여부 판단
- 안전 게이트
- report 생성

결과는 화면에 그대로 표시한다.

### 3. review

사용자는:

- rename 예정 목록
- skip 이유
- unresolved 현황
- AI가 개입한 항목

을 검토한다.

### 4. apply

사용자가 `적용`을 누르면:

- apply 직전 snapshot 재검증
- 변경 감지 시 적용 거부 및 새로고침 유도
- 변경이 없으면 `ApplyPreview()` 수행

### 5. 완료 표시

적용 후에는:

- 적용 완료 건수
- skip 수
- 실패 시 에러 메시지

를 보여준다.

## 안정성 체크리스트

GUI 도입 후에도 아래는 변하면 안 된다.

- rename target 충돌 방지
- 같은 source 중복 rename 방지
- 기존 파일/디렉터리 덮어쓰기 방지
- case/normalization alias 안전성 유지
- 순환 rename 처리와 rollback
- AI 병합 후 재안전검사

즉, GUI는 UX만 추가할 뿐, 핵심 안전성은 계속 `RenamePlan`과 `ApplyRenamePlan()`이 책임져야 한다.

## 테스트 전략

### 기존 테스트 유지

현재 rule/plan/safety 테스트는 그대로 유지해야 한다.

GUI를 붙인다고 해서 `pkg/re` 테스트를 건너뛰면 안 된다.

### 서비스 계층 테스트 추가

새로 분리한 `BuildPreview()`와 `ApplyPreview()`에 대해 아래를 검증한다.

- CLI preview와 같은 plan이 만들어지는가
- AI on/off에 따라 예상한 report가 나오는가
- preview 후 파일 상태가 바뀌면 apply가 거부되는가

### GUI는 얇게 검증

GUI는 복잡한 통합 테스트보다 아래 정도면 충분하다.

- preview 결과가 표와 skip 목록에 매핑되는가
- apply 버튼이 preview 없이는 비활성화되는가
- 에러 시 다이얼로그가 뜨는가

## 실행 체크리스트

### 단계 0. 고정 전제

- [x] 1차 GUI 프레임워크는 `Fyne`로 간다.
- [x] GUI는 `pkg/re`를 직접 호출하고, rename 규칙을 자체 구현하지 않는다.
- [x] destination 수동 편집, 개별 행 제외, 백업/undo UI는 1차 범위에서 제외한다.
- [x] GUI에서도 preview 없이 바로 apply하지 않는다.
- [x] GUI에서도 apply는 최종 사용자 확인 뒤에만 수행한다.

완료 조건:

- 문서만 읽어도 이번 구현의 범위와 비범위를 오해 없이 설명할 수 있어야 한다.

### 단계 1. 서비스 계층 설계

- [x] `PreviewRequest` 타입을 추가한다.
- [x] `PreviewResult` 타입을 추가한다.
- [x] `DirectorySnapshot`과 `FileSnapshot` 타입을 추가한다.
- [x] `BuildPreview(ctx, req)` 시그니처를 확정한다.
- [x] `ApplyPreview(preview)` 시그니처를 확정한다.
- [x] 공용 서비스 계층이 `stdin/stdout`에 의존하지 않도록 경계를 정리한다.
- [x] GUI와 CLI가 공통으로 쓸 에러/경고 모델을 정리한다.

완료 조건:

- CLI와 GUI가 같은 요청/응답 타입으로 preview/apply를 호출할 수 있어야 한다.
- 서비스 계층은 UI 프레임워크와 무관해야 한다.

### 단계 2. preview 생성 경로 분리

- [x] `ScanDirectory()` 결과를 서비스 계층으로 묶는다.
- [x] `ResolveByRule()` 결과를 서비스 계층으로 묶는다.
- [x] `BuildRenamePlan()`과 `EnforceSafeRenamePlan()`을 서비스 계층에 연결한다.
- [x] AI fallback on/off에 따른 preview 경로를 서비스 계층에 통합한다.
- [x] `BuildRunReport()`를 preview 결과에 포함한다.
- [x] preview 결과에 operations, skips, summary가 모두 들어오도록 정리한다.

완료 조건:

- 기존 CLI preview와 동일한 rename 계획이 서비스 계층에서도 나와야 한다.
- AI fallback 활성화 여부가 service API 입력만으로 제어되어야 한다.

### 단계 3. apply 경로와 재검증

- [x] preview 시점 디렉터리 스냅샷을 생성한다.
- [x] apply 직전 스냅샷 비교 함수를 추가한다.
- [x] source 파일이 사라졌을 때 apply를 거부한다.
- [x] destination 상태가 preview 시점과 달라졌을 때 apply를 거부한다.
- [x] 스냅샷 불일치 시 "새로고침 필요" 오류를 구분해서 반환한다.
- [x] 실제 rename 적용은 계속 `ApplyRenamePlan()`만 사용하게 유지한다.

완료 조건:

- 사용자가 preview 이후 디렉터리를 바꾸면 GUI/CLI 모두 안전하게 적용을 멈춰야 한다.
- preview 결과와 다른 plan이 apply에 사용되지 않아야 한다.

### 단계 4. CLI 재배선

- [x] [main.go](/Users/zrma/code/src/re/cmd/re/main.go) 경로의 CLI가 새 서비스 계층을 사용하도록 바꾼다.
- [x] 기존 `-t` 기본 동작을 유지한다.
- [x] 기존 `--yes` 동작을 유지한다.
- [x] 기존 `--output text/json` 동작을 유지한다.
- [x] 기존 `--ai-*` 옵션 동작을 유지한다.
- [x] 기존 preview 출력 형식을 유지한다.
- [x] 기존 확인 프롬프트 흐름을 유지한다.
- [x] 기존 cancel/applied/noop/needs_review 상태 계산을 유지한다.

완료 조건:

- CLI 사용자 입장에서 옵션과 출력 의미가 바뀌지 않아야 한다.
- CLI는 새 서비스 계층을 호출하는 얇은 래퍼가 되어야 한다.

### 단계 5. 서비스 계층 테스트

- [x] rule-only 케이스에서 기존과 동일한 operations/skips가 생성되는지 테스트한다.
- [x] AI fallback 케이스에서 기존과 동일한 병합 결과가 생성되는지 테스트한다.
- [x] noop 케이스 보고서가 유지되는지 테스트한다.
- [x] needs_review 케이스 보고서가 유지되는지 테스트한다.
- [x] canceled/apply 상태가 기존 의미를 유지하는지 테스트한다.
- [x] preview 후 파일이 바뀌면 apply가 거부되는지 테스트한다.
- [x] 순환 rename이 여전히 동작하는지 기존 테스트로 유지한다.
- [x] rollback 경로가 유지되는지 기존 테스트로 유지한다.

완료 조건:

- GUI 추가 전에도 공용 서비스 계층만으로 핵심 동작을 검증할 수 있어야 한다.

### 단계 6. Fyne GUI 스켈레톤

- [x] `cmd/re-gui/main.go`를 추가한다.
- [x] 앱 시작 창을 만든다.
- [x] 대상 폴더 선택 UI를 만든다.
- [x] 대상 폴더 경로를 직접 붙여넣고 Enter로 preview를 생성할 수 있게 한다.
- [x] AI fallback 옵션 UI를 만든다.
- [x] 경로나 AI 옵션이 바뀌면 이전 preview를 stale 상태로 취급해 apply를 막는다.
- [x] 새로고침 버튼을 만든다.
- [x] loading 상태를 표시한다.
- [x] 에러 상태를 표시한다.
- [x] preview 결과가 없을 때 apply 버튼을 비활성화한다.
- [x] preview refresh 준비 단계에서 기존 preview를 비우고 apply를 막는 회귀 테스트를 추가한다.
- [x] idle/empty state 표시가 깨지지 않도록 기본 회귀 테스트를 추가한다.

완료 조건:

- 사용자가 폴더를 고르고 preview를 요청할 수 있어야 한다.
- preview 전에는 apply가 불가능해야 한다.

### 단계 7. preview 화면

- [x] operations 테이블을 만든다.
- [x] 테이블에 source 컬럼을 표시한다.
- [x] 테이블에 destination 컬럼을 표시한다.
- [x] 테이블에 match source 컬럼을 표시한다.
- [x] AI confidence를 표시한다.
- [x] skip 목록을 별도 패널로 표시한다.
- [x] skip 목록을 source/reason 2열로 읽기 쉽게 정리한다.
- [x] summary를 별도 패널로 표시한다.
- [x] noop 상태를 읽기 쉽게 표시한다.
- [x] needs_review 상태를 읽기 쉽게 표시한다.
- [x] 긴 파일명은 중간 말줄임과 선택 상세 패널로 검증 가능하게 한다.
- [x] AI match는 낮은 confidence부터 먼저 검토할 수 있게 정렬한다.
- [x] warning/error 상태를 일반 텍스트가 아니라 별도 배너로 구분한다.
- [x] preview, selection, skips, summary를 분리된 섹션과 요약 지표로 정리해 정보 계층을 분명히 한다.

완료 조건:

- 사용자가 "무엇이 rename되고 무엇이 skip되는지"를 한 화면에서 판단할 수 있어야 한다.

### 단계 8. apply UX

- [x] `적용` 버튼 클릭 시 최종 확인 다이얼로그를 띄운다.
- [x] 확인 다이얼로그에 rename 건수와 skip 건수를 보여준다.
- [x] apply 중 버튼 중복 클릭을 막는다.
- [x] apply 성공 시 완료 메시지를 보여준다.
- [x] apply 실패 시 에러 메시지를 보여준다.
- [x] 스냅샷 불일치 시 새로고침 유도 메시지를 보여준다.
- [x] apply 이후 화면을 새 상태로 갱신한다.
- [x] apply 확인 다이얼로그에 active warning을 함께 보여준다.
- [x] stale preview로 apply가 거부되면 기존 preview를 비우고 다시 생성 전까지 apply를 막는다.

완료 조건:

- 사용자가 의도치 않게 중복 apply할 수 없어야 한다.
- 실패 시 왜 실패했는지 다음 행동이 분명해야 한다.

### 단계 9. 패키징과 배포

- [x] 로컬에서 GUI 바이너리 빌드 방법을 정리한다.
- [x] GUI 패키징을 release workflow에 바로 포함할지 결정한다.
- [x] 현재 단계에서는 release workflow를 바꾸지 않기로 정리한다.
- [x] CLI 릴리즈와 GUI 로컬 실행을 분리 운영하는 방침을 문서화한다.
- [x] GUI는 데스크톱 세션 전제로 운영하고, headless 환경은 범위 밖이라고 문서화한다.

완료 조건:

- "어떻게 실행하고 배포하는지"가 모호하지 않아야 한다.

### 단계 10. 문서와 사용 가이드

- [x] README에 GUI 실행 방법을 추가한다.
- [x] README에 GUI의 안전 원칙을 추가한다.
- [x] README 코드 구조 섹션에 GUI 경로를 추가한다.
- [x] GUI 제한 사항을 README나 별도 문서에 적는다.
- [ ] 필요하면 스크린샷을 추가한다.

완료 조건:

- 처음 보는 사용자가 GUI의 역할과 제약을 README만 읽고 이해할 수 있어야 한다.

## 수동 QA 체크리스트

- [ ] 단순 1:1 rule 매칭 폴더에서 preview와 apply가 정상 동작한다.
- [ ] rename target 충돌 케이스에서 GUI가 skip 이유를 정확히 보여준다.
- [ ] 기존 파일이 destination에 이미 있는 경우 apply가 막힌다.
- [ ] 내부 임시 파일(`.re-tmp-*`)이 남아 있는 폴더에서 경고/skip이 보인다.
- [ ] 순환 rename 케이스가 실제로 적용된다.
- [ ] AI fallback 케이스에서 `[ai]` 출처가 화면에 구분되어 보인다.
- [ ] 낮은 confidence AI 항목이 preview 상단으로 올라오는지 확인한다.
- [ ] unresolved/needs_review 상태가 사용자가 이해할 수 있게 보인다.
- [ ] preview 후 사용자가 폴더를 변경하면 apply가 거부된다.
- [ ] preview 생성 후 경로나 AI 옵션을 바꾸면 apply가 비활성화되고 재생성을 요구한다.
- [ ] preview 생성 실패 또는 apply 실패 후 경로나 옵션을 바꾸면 이전 오류 배너가 사라지고 재생성 안내가 보인다.
- [ ] 빈 폴더 또는 지원 파일이 없는 폴더에서 앱이 깨지지 않는다.
- [ ] 취소 경로에서 실제 파일 변경이 일어나지 않는다.

## 최종 검증 체크리스트

- [x] `go test ./...`
- [x] `go test -race ./...`
- [x] CLI 스모크 테스트
- [ ] GUI 스모크 테스트
- [x] release workflow 영향 범위 검토
- [x] README/설계 문서 최신화 확인

## 이번 작업의 종료 조건

- [x] CLI와 GUI가 같은 서비스 계층을 사용한다.
- [x] GUI가 preview, skip 이유, summary, apply 확인을 제공한다.
- [x] preview 이후 디렉터리 변경 시 apply를 막는다.
- [ ] 기존 안전성 테스트와 수동 QA를 통과한다.
- [x] 문서가 실제 구현 상태와 일치한다.

## Gemini CLI의 위치

Gemini CLI는 GUI 아키텍처의 중심이 아니라 보조 도구로 보는 편이 맞다.

- 화면 문구 초안 작성
- 레이아웃 아이디어 제안
- Fyne 화면 코드 초안 생성
- 아이콘/문구/상태 표현 다듬기
- UX 리뷰와 누락된 안전 검토 포인트 재확인

에는 유용하다.

하지만 안정성 경계, rename plan 구조, apply 검증, 파일 시스템 안전장치는 계속 로컬 Go 코드가 책임져야 한다.

## 오픈 이슈

- apply 직전 스냅샷은 `size + mtime`로 충분한가
- GUI 전용 실행 옵션 타입을 둘지, 기존 `RunOptions`를 그대로 확장할지
- GUI 패키징을 release workflow에 바로 포함할지, CLI와 분리할지
- macOS에서 파일 접근 권한 안내를 별도로 둘지
- 행 단위 제외/개별 apply는 정말 필요한가
  현재는 명시적으로 비목표이며, 도입 시 `PreviewResult`/`ApplyPreview()` 계약부터 다시 설계해야 한다.

## 결론

첫 GUI는 "예쁜 데스크톱 앱"보다 "기존 rename 엔진을 안전하게 감싼 도구형 화면"으로 시작하는 편이 맞다.

이 프로젝트에서는:

- 핵심 로직을 `pkg/re` 공용 서비스로 정리하고
- CLI를 그 위로 재배선하고
- `Fyne`로 얇은 GUI를 붙이는 순서가

가장 작은 변경으로 가장 높은 안정성을 얻는 경로다.
