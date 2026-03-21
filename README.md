# re

같은 폴더에 있는 영상 파일명에 맞춰 자막 파일명을 일괄 정리하는 Go CLI입니다.

[![CI](https://github.com/zrma/re/actions/workflows/ci.yml/badge.svg)](https://github.com/zrma/re/actions/workflows/ci.yml)

애니메이션 릴리스처럼 영상 파일명과 자막 파일명이 서로 다를 때, 파일명에서 에피소드 번호를 추출해 자막명을 영상명 기준으로 바꿉니다.

예를 들어 아래처럼 영상과 자막의 naming rule이 달라도:

```text
[Moozzi2] Shigatsu wa Kimi no Uso - 01 (BD 1920x1080 x.264 FLACx2).mkv
1화.srt
```

`re`는 에피소드 `01`을 기준으로 매칭해서 아래처럼 바꿉니다.

```text
[Moozzi2] Shigatsu wa Kimi no Uso - 01 (BD 1920x1080 x.264 FLACx2).srt
```

## 무엇을 하는 프로젝트인가

이 프로젝트는 다음 상황을 해결합니다.

- 영상 파일은 BD/WEB 릴리스 이름을 유지하고 싶다.
- 자막 파일은 `1화.srt`, `카노카리 03.smi`, `[Ohys-Raws] ... - 07 ... .ass`처럼 제각각이다.
- 자막 플레이어가 자동 매칭되도록 자막 파일명을 영상 파일명과 동일한 basename으로 맞추고 싶다.

`re`는 대상 디렉터리의 바로 아래 파일만 스캔한 뒤, 영상과 자막에서 에피소드 번호를 뽑아 매칭하고, 변경 예정 목록을 먼저 보여준 다음 확인을 받아 실제 rename을 수행합니다.

## 동작 방식

1. 대상 폴더의 확장자를 먼저 소문자로 정규화합니다.
   예: `.MKV` -> `.mkv`, `.SRT` -> `.srt`
2. 대상 폴더의 바로 아래 파일 중 지원하는 영상/자막 파일만 찾습니다.
3. 각 파일명에서 에피소드 번호를 추출합니다.
4. 영상 basename을 기준으로 자막의 새 이름을 계산합니다.
5. 변경 예정 목록을 출력한 뒤 `y`를 입력하면 실제로 rename합니다.

기본 대상 경로는 현재 디렉터리(`.`)이며, `-t` 옵션으로 다른 경로를 지정할 수 있습니다.

## 지원하는 확장자

- 영상: `.avi`, `.mkv`, `.mp4`
- 자막: `.srt`, `.ass`, `.smi`

## 에피소드 추출 규칙

파일명에서 아래 패턴 중 하나를 찾으면 해당 값을 에피소드 번호로 사용합니다.

| 패턴 | 예시 | 추출 결과 |
| --- | --- | --- |
| `OAD` | `My Title OAD.mkv` | `OAD` |
| `E01` | `abcde E01 [1080p].mkv` | `01` |
| `1x01` | `Show 1x01.mkv` | `01` |
| `第01話` | `スローループ 第01話.mkv` | `01` |
| `- 01 (` | `Title - 01 (BD 1920x1080).mkv` | `01` |
| `S01_E01` | `Title S01_E01.mkv` | `01` |
| `1화` | `1화.srt` | `01` |
| `01.` | `Title 01.ass` | `01` |
| `- 01 RAW` | `Title - 01 RAW.smi` | `01` |

추출은 위 순서대로 시도합니다. 지원하지 않는 파일명 패턴이면 해당 파일은 건너뛰고 로그만 남깁니다.

## 사용법

### 실행

```bash
go run ./cmd/re -t /path/to/subtitle-folder
```

현재 디렉터리를 대상으로 실행하려면:

```bash
go run ./cmd/re
```

바이너리를 직접 빌드해서 사용할 수도 있습니다.

```bash
go build -o re ./cmd/re
./re -t /path/to/subtitle-folder
```

확인 프롬프트 없이 바로 적용하려면:

```bash
go run ./cmd/re -t /path/to/subtitle-folder --yes
```

JSON 보고 형식으로 결과를 받으려면:

```bash
go run ./cmd/re -t /path/to/subtitle-folder --yes --output json
```

### 실행 예시

폴더 안에 아래 파일이 있을 때:

```text
[Moozzi2] Eureka Seven AO - 01 (BD 1920x1080 x.264 FLACx2).mkv
[Leopard-Raws] Eureka Seven Ao - 01 RAW (TBS 1280x720 x264 AAC).smi
```

실행하면 먼저 dry-run 형태로 변경 예정 목록을 보여줍니다.

```text
/path/to/subtitle-folder/[Leopard-Raws] Eureka Seven Ao - 01 RAW (TBS 1280x720 x264 AAC).smi -> [Moozzi2] Eureka Seven AO - 01 (BD 1920x1080 x.264 FLACx2).smi
Summary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0
Do you want to rename? (y/n)
```

여기서 `y`를 입력하면 rename이 적용되고, 그 외 입력이면 취소됩니다.

## 제약 사항

- 하위 디렉터리는 재귀적으로 처리하지 않습니다.
  대상 경로의 바로 아래 파일만 봅니다.
- 에피소드 번호를 추출하지 못한 파일은 rename하지 않습니다.
- 같은 에피소드에 대응되는 영상 파일이 여러 개 있으면 마지막으로 선택된 이름이 사용됩니다.
- 같은 에피소드에 같은 확장자의 자막이 여러 개 있으면 rename 대상 경로가 충돌할 수 있습니다.
- 실제 실행 전에 별도 백업을 만들지 않습니다.

## AI fallback 구상

rule 기반 매칭으로 처리되지 않는 예외 케이스를 위해, `codex exec`를 보조 판정기로 붙이는 하이브리드 구조를 고려할 수 있습니다.

- 정상 케이스는 기존 deterministic 매칭 유지
- unresolved 케이스만 AI에 질의
- AI는 rename을 직접 하지 않고 추천 매핑안만 반환
- 최종 rename은 로컬 Go 코드가 안전하게 수행

### AI fallback 실행 예시

기본 동작은 기존과 같습니다.

```bash
go run ./cmd/re -t /path/to/subtitle-folder
```

unresolved subtitle에 대해서만 AI 보조 판정을 켜려면:

```bash
go run ./cmd/re \
  -t /path/to/subtitle-folder \
  --yes \
  --output json \
  --ai-fallback \
  --ai-model gpt-5.4-mini \
  --ai-min-confidence 0.90
```

기본 `text` 출력은 출처, skip 이유, 요약 통계를 함께 보여줍니다.

```text
/path/to/subtitle-folder/1화.srt -> [BD] Example - 01.srt
[ai:0.98] /path/to/subtitle-folder/oad-kor.srt -> [BD] Example OAD.srt
[skip] /path/to/subtitle-folder/commentary-kor.srt (ai requested human review: ambiguous between OAD and TV special)
Summary: 2 renames (rule 1, ai 1), 1 skips, unresolved movies 0, unresolved subtitles 2
Do you want to rename? (y/n)
```

`--yes`를 함께 주면 preview는 유지되지만 확인 프롬프트는 생략됩니다.

`--output json`을 주면 stdout은 구조화된 JSON만 출력합니다. 자동화나 후처리에는 이 모드를 쓰는 편이 좋습니다.

```json
{
  "target_path": "/path/to/subtitle-folder",
  "status": "applied",
  "applied": true,
  "requires_confirmation": false,
  "summary": {
    "movies_total": 2,
    "subtitles_total": 3,
    "planned_renames": 2,
    "rule_renames": 1,
    "ai_renames": 1,
    "skips": 1,
    "unresolved_movies": 0,
    "unresolved_subtitles": 2
  }
}
```

### AI debug 출력

`--ai-debug-output`을 주면 `codex exec` 요청/응답을 실행별 번들 디렉터리로 저장합니다.

```bash
go run ./cmd/re \
  -t /path/to/subtitle-folder \
  --ai-fallback \
  --ai-debug-output ./debug/re
```

예시 구조:

```text
debug/re/
  run-20260322T001122.123456789Z/
    metadata.json
    request/
      input.json
      prompt.txt
      schema.json
    response/
      output.json
      stderr.log
```

이 출력은 로컬 스모크 테스트나 실패 재현용으로 두는 것이 좋고, CI artifact나 정규 테스트 fixture로 묶지는 않는 편이 안전합니다.

자세한 설계는 [`docs/ai-fallback-design.md`](docs/ai-fallback-design.md)를 참고하세요.

## 테스트와 예제 데이터

테스트는 CSV fixture 기반으로 여러 naming rule을 검증합니다.

- 한국어 자막명: `1화.srt`, `카노카리 03.smi`
- 시즌 표기: `S01_E01`
- 일본어 표기: `第01話`
- RAW 릴리스 표기: `- 01 RAW`

검증 실행:

```bash
go test ./...
```

주요 테스트 파일:

- `test/re_test.go`
- `test/testdata/normal.csv`
- `test/testdata/kanokari.csv`

## 코드 구조

- `cmd/re/main.go`: CLI 진입점, `-t`, `--yes`, `--ai-*` 플래그 처리
- `pkg/re/re.go`: 실행 오케스트레이션, preview, 확인/적용 흐름 제어
- `pkg/re/report.go`: text/json 출력과 요약 통계 생성
- `pkg/re/scan.go`: 대상 디렉터리 스캔과 파일 분류
- `pkg/re/resolve.go`: rule 기반 episode 해석과 unresolved 분류
- `pkg/re/plan.go`: rename plan과 skip plan 생성, preview 출력
- `pkg/re/options.go`: 실행 옵션과 AI 옵션 기본값
- `pkg/re/ai.go`: AI 입력/출력 모델, merge 로직, skip 이유 정리
- `pkg/re/codex_exec.go`: `codex exec` 기반 AI resolver
- `pkg/re/codex_debug.go`: 실행별 debug bundle 저장
- `pkg/re/extract.go`: 에피소드 추출 체인 정의
- `pkg/re/parse.go`: 파일명 패턴별 파서 구현
