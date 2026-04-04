# AI Fallback 설계

## 요약

이 프로젝트의 기본 철학은 유지한다.

- 1차 매칭은 지금처럼 deterministic rule 기반으로 처리한다.
- rule 기반으로 확정할 수 없거나, 안전 게이트에서 적용 불가로 걸러진 자막만 AI에 질의한다.
- AI는 실제 rename을 수행하지 않고, "추천 매핑안"만 반환한다.
- 최종 rename은 로컬 Go 코드가 안전장치를 거쳐 수행한다.

즉, `re`를 "AI가 알아서 파일을 건드리는 도구"로 바꾸는 것이 아니라, "기존 rename 도구에 지능형 fallback을 붙이는 것"이 목표다.

## 왜 하이브리드 구조인가

현재 프로젝트가 다루는 문제는 다음 특성이 강하다.

- 성공 케이스가 반복적이다.
- 결과가 항상 같아야 한다.
- 한번 잘못 rename하면 되돌리기 번거롭다.

이런 문제는 rule 기반이 여전히 강하다. 반면 아래 상황은 rule만으로 불편하다.

- 파일명 패턴이 생소해서 episode 추출이 안 되는 경우
- 시즌/스페셜/OAD/NCOP/NCED 표기가 섞인 경우
- 자막 후보와 영상 후보가 1:1로 딱 떨어지지 않는 경우
- 파일명이 여러 언어로 섞여 있어 정규식 추가가 끝없이 늘어나는 경우

따라서 AI는 "예외 처리기"로만 사용한다.

## 목표

- 기존 deterministic 성공 케이스의 동작을 절대 깨지 않는다.
- 매칭 실패 케이스를 더 많이 처리한다.
- 잘못된 rename보다 "skip"을 우선한다.
- AI 결과는 설명 가능하고 review 가능해야 한다.
- dry-run과 사용자 확인 흐름은 유지한다.

## 비목표

- AI가 파일 시스템을 직접 조작하게 하지 않는다.
- 영상/자막 파일 내용 분석까지 확장하지 않는다.
- 하위 디렉터리 재귀 처리까지 한 번에 풀지 않는다.
- 첫 단계에서 완전 자동 무인 rename을 목표로 하지 않는다.

## 제안하는 사용자 경험

기본 동작은 그대로 둔다.

```bash
go run ./cmd/re -t /path/to/folder
```

AI fallback을 켜면 rule만으로 확정 적용하지 못한 케이스만 추가 분석한다.

```bash
go run ./cmd/re -t /path/to/folder --ai-fallback
```

추천 옵션:

```bash
go run ./cmd/re \
  -t /path/to/folder \
  --yes \
  --output json \
  --ai-fallback \
  --ai-model gpt-5.4-mini \
  --ai-min-confidence 0.90
```

`--ai-fallback` 없이도 기존 동작은 100% 유지되어야 한다.

## 제안 CLI 옵션

- `--ai-fallback`
  unresolved 또는 안전 게이트에서 걸러진 subtitle이 있을 때만 AI 보조 매칭을 수행한다.
- `--ai-model`
  `codex exec`에 넘길 모델 이름. 예: `gpt-5.4-mini`
- `--ai-min-confidence`
  이 값 미만 응답은 적용하지 않는다. 기본값 권장: `0.90`
- `--ai-debug-output`
  AI 요청/응답 payload를 파일로 저장한다.
- `--ai-timeout`
  외부 호출 최대 대기 시간
- `--yes`
  최종 rename 확인 프롬프트를 생략한다.
- `--output`
  `text` 또는 `json` 형식으로 결과를 출력한다.

추가로 나중에 고려할 옵션:

- `--ai-only-unresolved`
  기본값으로 사실상 동일하지만 의미를 명시하는 옵션

## 상위 아키텍처

### 1. 스캔 단계

현재처럼 대상 디렉터리 바로 아래에서 영상/자막 파일을 수집한다.

여기서 단순 문자열 배열이 아니라 구조화된 레코드로 바꾸는 것이 좋다.

```go
type MediaFile struct {
    Path      string
    BaseName  string
    Extension string
    Kind      string // movie | subtitle
}
```

### 2. Deterministic 매칭 단계

기존 `extractEpisode()`와 parser 체인을 그대로 활용한다.

결과는 다음 세 그룹으로 나눈다.

- `resolved`
  규칙 기반으로 확정된 rename 계획
- `unresolved`
  에피소드를 못 뽑았거나 후보가 애매한 항목
- `conflicted`
  같은 목적지 파일명으로 여러 자막이 몰리는 등 안전하지 않은 항목

### 3. AI 보조 매칭 단계

`--ai-fallback`이 켜져 있고 `unresolved`가 비어 있지 않을 때만 `codex exec`를 호출한다.

AI에는 전체 파일 시스템 권한을 주지 않는다. 아래 정보만 넘긴다.

- 대상 디렉터리 basename 목록
- 영상 후보 목록
- unresolved 자막 목록
- deterministic 단계에서 이미 판정한 결과 일부
- 매칭 규칙 요약

중요한 점은, AI에 "rename 명령"을 내리지 않고 "구조화된 판정 결과"만 받는 것이다.

### 4. 안전 게이트

AI 응답을 그대로 적용하지 않고 로컬 코드가 아래 검증을 통과시킨 뒤에만 rename 후보로 승격한다.

- 반환된 `matched_movie`가 실제로 존재하는가
- subtitle 1개가 movie 1개로만 연결되는가
- rename 결과 경로 충돌이 없는가
- confidence가 기준치 이상인가
- outcome이 `match`인가
- AI가 지정한 basename이 아닌, 실제 movie basename만 목적지로 사용하는가

하나라도 실패하면 `skip` 처리한다.

### 5. Preview 단계

사용자에게 아래처럼 출처를 구분해서 보여준다.

```text
[rule] /subs/1화.srt -> [Moozzi2] ... 01.srt
[ai:0.96] /subs/special-kor.srt -> [BD] Title OAD.srt
[skip] /subs/extra-commentary.ass (low confidence: 0.62)
```

### 6. Rename 단계

기본적으로는 사용자 확인 이후에만 rename한다. `--yes`가 켜진 경우에만 preview 후 즉시 적용한다. `--output json`을 쓰면 결과는 구조화된 보고서로 출력한다.

실제 파일 조작은 계속 Go 코드가 맡는다.

## `codex exec` 호출 방식

비대화형 호출을 사용한다.

예시:

```bash
codex exec \
  --skip-git-repo-check \
  --sandbox read-only \
  --model gpt-5.4-mini \
  --output-schema /tmp/re-ai-schema.json \
  -o /tmp/re-ai-response.json \
  -
```

stdin에는 프롬프트와 입력 JSON을 넣는다.

이 구조의 장점:

- 모델이 로컬 파일 rename을 직접 실행하지 못한다.
- 최종 응답 형식을 JSON Schema로 제한할 수 있다.
- Go 쪽에서는 stdout 파싱 대신 구조화된 파일 입력을 읽으면 된다.

모델 이름은 환경에 따라 달라질 수 있으므로 코드상 기본값은 설정하되, 항상 flag로 override 가능하게 두는 편이 안전하다.

## AI 입력 payload 초안

```json
{
  "directory": "Downloads",
  "movies": [
    {
      "path": "/path/[BD] Title - 01.mkv",
      "basename": "[BD] Title - 01"
    }
  ],
  "subtitles": [
    {
      "path": "/path/1화.srt",
      "basename": "1화",
      "extension": ".srt"
    }
  ],
  "unresolved_subtitles": [
    "/path/special-kor.srt"
  ],
  "resolved_pairs": [
    {
      "subtitle": "/path/1화.srt",
      "movie": "/path/[BD] Title - 01.mkv",
      "source": "rule"
    }
  ],
  "rules": {
    "must_preserve_extension": true,
    "must_use_existing_movie_basename": true,
    "prefer_skip_over_guess": true
  }
}
```

## AI 출력 schema 초안

```json
{
  "type": "object",
  "required": ["decisions"],
  "properties": {
    "decisions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": [
          "subtitle_path",
          "outcome",
          "matched_movie_path",
          "confidence",
          "reason"
        ],
        "properties": {
          "subtitle_path": { "type": "string" },
          "outcome": {
            "type": "string",
            "enum": ["match", "skip", "needs_human"]
          },
          "matched_movie_path": {
            "type": ["string", "null"]
          },
          "confidence": {
            "type": "number",
            "minimum": 0,
            "maximum": 1
          },
          "reason": { "type": "string" }
        }
      }
    }
  }
}
```

`reason`은 사람이 preview를 볼 때 판단할 수 있도록 짧게 남긴다.

예:

- `same title token and OAD marker`
- `likely season 1 episode 03 based on nearby resolved pairs`
- `ambiguous between TV episode and BD special`

## 프롬프트 원칙

프롬프트는 최대한 보수적으로 잡는다.

- 추측보다 skip을 우선한다.
- 실제로 존재하는 movie 후보 중 하나만 선택한다.
- 없는 파일명을 발명하지 않는다.
- subtitle extension은 유지한다.
- confidence는 엄격하게 부여한다.
- 스페셜/OAD/NCOP/NCED/OVA/OAD는 일반 본편과 혼동하지 않는다.

예시 지시문:

```text
You are assisting a local subtitle renamer.
Return JSON only.
Do not invent filenames.
If uncertain, choose "skip" or "needs_human".
Only match a subtitle to an existing movie path from the provided list.
Prefer precision over recall.
```

## 안전성 원칙

이 기능의 핵심은 AI 사용 여부가 아니라 "AI가 틀려도 시스템이 위험해지지 않게 하는 것"이다.

- `codex exec`는 read-only sandbox로 실행
- 출력은 JSON Schema로 제한
- low confidence 자동 거절
- 경로 충돌 자동 거절
- dry-run preview 기본 유지
- 최종 rename은 사용자 확인 후 수행
- 필요 시 `--ai-debug-output`으로 추적 가능

## 구현 분해안

### 1단계: 내부 모델 정리

현재 `Run()`은 스캔, 판정, 출력, rename이 한 함수에 몰려 있다.

먼저 아래처럼 나누는 것이 좋다.

- `ScanDirectory(targetPath) -> ScanResult`
- `ResolveByRule(scanResult) -> ResolutionResult`
- `BuildRenamePlan(...) -> RenamePlan`
- `ApplyRenamePlan(plan, dryRun)`

이 단계만 해도 AI 없이 테스트하기 쉬워진다.

### 2단계: AI resolver 인터페이스 도입

```go
type AIResolver interface {
    Resolve(ctx context.Context, input AIInput) (AIOutput, error)
}
```

기본 구현:

- `CodexExecResolver`

테스트용 구현:

- `FakeResolver`

### 3단계: 하이브리드 merge 로직 추가

deterministic 결과와 AI 결과를 합쳐 최종 plan을 만든다.

우선순위는 아래처럼 둔다.

1. rule로 확정된 결과
2. AI가 high confidence로 제안한 결과
3. 그 외는 skip

### 4단계: CLI flag 연결

`cmd/re/main.go`에서 새 flag를 받고 `re.Run()` 또는 새 orchestration 함수에 전달한다.

### 5단계: 테스트 추가

최소 테스트 세트:

- 기존 fixture가 그대로 통과하는지
- unresolved만 AI에 전달되는지
- AI가 없는 파일명을 반환하면 거절되는지
- low confidence 응답이 skip 처리되는지
- path collision이 거절되는지
- preview에 `[rule]`, `[ai]`, `[skip]`가 구분되어 출력되는지

## 추천 롤아웃 순서

1. 먼저 내부 로직을 pure function 중심으로 재구성한다.
2. `--ai-fallback` 없이도 기존 테스트가 그대로 통과하게 유지한다.
3. 그 다음 `FakeResolver`로 merge 로직 테스트를 붙인다.
4. 마지막에만 `CodexExecResolver`를 추가한다.
5. 초기 릴리스에서는 `--ai-fallback`을 opt-in으로만 연다.

## 판단

이 프로젝트는 AI로 완전히 대체하기보다, deterministic rename 엔진에 AI fallback을 붙일 때 가장 큰 효과를 얻는다.

정리하면:

- 기본 매칭: 기존 정규식/규칙 기반 유지
- 실패 케이스: `codex exec`로 보조 판단
- 실제 rename: 로컬 Go 코드만 수행
- low confidence: 과감히 skip

이 방향이면 "영리함"은 얻고, rename 도구가 가져야 할 안정성도 유지할 수 있다.
