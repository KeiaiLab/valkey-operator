# ADR-0044: Artifact Hub trust badges — Signed mandatory, Official external review

- Date: 2026-05-12
- Status: Accepted
- Authors: @eightynine01
- Refs: ADR-0024, ADR-0033, `docs/operations/artifacthub-trust.md`

## Context

Artifact Hub 의 현재 패키지 상태는 repository verified publisher 는 통과했지만
package `signed=false`, repository `official=false` 이다. gh-pages 에 제공되는
`valkey-operator-1.0.10.tgz.prov` 도 404 로 확인되어 Artifact Hub `Signed`
badge 가 켜질 수 없다.

기존 ADR-0024 는 `HELM_SIGN=1` 옵션을 제공했지만 기본값은 unsigned 였다.
이 상태에서는 release 담당자가 옵션을 빼먹어도 release 가 성공하므로 운영
신뢰 지표가 회귀한다.

반면 `Official` 은 Artifact Hub 가 정의한 외부 심사 상태다. repository 파일에
`official: true` 를 추가하는 방식으로 자체 선언할 수 없고, publisher 가 해당
소프트웨어를 소유한다는 조건으로 Artifact Hub 쪽 official status request 가
승인되어야 한다.

## Decision

1. Helm chart release 의 기본값을 `HELM_SIGN=1` 로 변경한다.
2. `make release` 와 `make helm-publish` 는 signed chart package 와
   `.tgz.prov` 생성을 전제로 한다.
3. Helm `--key` 값은 fingerprint 가 아니라 UID substring 이어야 하므로
   `HELM_GPG_KEY=Keiailab Helm` 을 기본값으로 둔다. fingerprint 는
   `HELM_GPG_FINGERPRINT` 로 분리해 문서/검증용으로만 사용한다.
4. release smoke test 는 GH Release asset, gh-pages provenance fetch,
   `artifacthub-repo.yml` 의 public signing key import, `helm verify` 까지
   수행한다.
5. `Official` 은 코드로 claim 하지 않는다. `docs/operations/artifacthub-trust.md`
   에 선행조건과 official status request 내용을 고정하고, Artifact Hub publisher
   권한을 가진 사용자가 외부 request 를 제출한다.

## Consequences

긍정:

- `Signed` badge 는 다음 정상 release 부터 release pipeline 의 기본 경로가 된다.
- `.tgz.prov` 누락이 GH Release 와 gh-pages 양쪽 smoke 에서 즉시 드러난다.
- Helm key fingerprint 를 `--key` 로 쓰는 잘못된 release 설정을 제거한다.
- `Official` 의 책임 경계가 명확해져, 코드 변경만으로 완료됐다고 오판하지 않는다.

부정:

- PGP secret key 가 없는 개발 장비에서는 `make release` 와 `make helm-publish` 가
  실패한다. 이는 의도된 fail-closed 동작이다.
- 기존 `v1.0.10` package 의 `signed=false` 는 private key 없이 이 세션에서 즉시
  뒤집을 수 없다. 기존 artifact 를 재서명하거나 새 patch release 를 signed 로
  publish 해야 한다.
- `Official` 은 Artifact Hub maintainer review 지연에 영향을 받는다.

## Alternatives Considered

1. **HELM_SIGN=0 유지 + release 담당자 수동 옵션** — 거부. 이전 상태가 실제로
   unsigned release 를 만들었고, 사람이 옵션을 기억해야 한다.
2. **새 PGP key 즉석 생성 후 publish** — 거부. 공식 release signing key 는
   회전/보관/폐기 정책이 필요하며, 임시 key 로 trust root 를 바꾸면 장기 검증성이
   떨어진다.
3. **Official 상태를 chart annotation 으로 자체 선언** — 거부. Artifact Hub
   official 은 외부 심사 상태이며, repository/package level grant 로 처리된다.

## Status

Accepted. 다음 release 는 `make helm-signing-preflight` 통과 후 signed chart 로
publish 해야 한다. Artifact Hub official request 는 외부 계정 권한을 가진
publisher 가 제출한다.
