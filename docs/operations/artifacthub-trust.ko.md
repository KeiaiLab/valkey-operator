# Artifact Hub Trust Badge 운영 — valkey-operator (한국어)

> English: [artifacthub-trust.md](artifacthub-trust.md) — canonical / 정본

Artifact Hub 의 `valkey-operator` 패키지에 대한 `Signed` / `Official` 배지를
획득·유지하기 위한 운영 절차. 본 문서는 *반복 가능한* 점검 + 신청 흐름을
SSOT 로 고정한다.

## 현황

2026-05-12 점검:

```bash
curl -fsSL https://artifacthub.io/api/v1/packages/helm/keiailab-valkey-operator/valkey-operator \
  | jq '{version, signed, official, repository:{verified_publisher:.repository.verified_publisher, official:.repository.official}}'
```

관측값:

- `repository.verified_publisher=true`
- `signed=false`
- `repository.official=false`
- `https://keiailab.github.io/valkey-operator/valkey-operator-1.0.10.tgz.prov` → 404

저장소 소유권 검증은 *이미 완료*. 남은 작업은 **Helm provenance 게시** 와
**Artifact Hub official status 심사** 두 축.

## Signed 배지 적용

Artifact Hub 는 chart archive 옆에 `<chart>-<version>.tgz.prov` 가
*함께 존재* 할 때에 한해 Helm `Signed` 상태를 노출한다. `.tgz.prov` 는
`helm package --sign` 으로 생성된다.

표준 release 명령:

```bash
make release VERSION=vX.Y.Z
```

`Makefile` 은 `HELM_SIGN=1` 을 기본값으로 강제하므로, 다음 사전조건이
*전부* 충족되지 않으면 release 가 실패한다:

- `~/.gnupg/secring.gpg`
- `HELM_GPG_KEY=Keiailab Helm`
- `HELM_GPG_FINGERPRINT=F1A6893583E632A757FF6767F3CC8C6AEC9CEB08`

신규 개발 머신에서 secret key 를 준비할 때:

```bash
gpg --import <keiailab-helm-private-key.asc>
gpg --export-secret-keys > ~/.gnupg/secring.gpg
make helm-signing-preflight
```

release 직후 검증:

```bash
bash scripts/release-smoke-test.sh vX.Y.Z
```

이 smoke test 는 (a) GitHub Releases 의 `.tgz.prov` asset, (b) `gh-pages`
브랜치의 `.tgz.prov` 파일, (c) `artifacthub-repo.yml` 의 공개 signing key 로
실행한 `helm verify` 결과를 *3 면 모두* 확인한다.

## Official 배지 적용

`Official` 은 저장소 안의 파일만으로는 켤 수 없다. Artifact Hub 가
*외부에서 심사* 하는 상태로, "publisher 가 해당 패키지가 다루는 소프트웨어를
직접 소유한다" 는 사실을 가리킨다.

이미 충족된 사전조건:

- Artifact Hub repository ID:
  `16085dd0-0f19-4c6b-ab90-bd97105bdf42`
- Verified publisher: `true`
- Chart README: `charts/valkey-operator/README.md`
- 저장소 메타데이터 게시:
  `https://keiailab.github.io/valkey-operator/artifacthub-repo.yml`

남은 단계:

1. Artifact Hub publisher 또는 organization 멤버가 `artifacthub/hub` 에
   official status request issue 를 연다.
2. **repository 단위** official status 를 요청한다.
3. 신청 본문에 다음 사실을 *그대로* 포함한다.

```text
Repository name: keiailab-valkey-operator
Package name: valkey-operator
Repository URL: https://keiailab.github.io/valkey-operator
Package URL: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
Publisher owns the software: yes, keiailab publishes and maintains valkey-operator itself.
Verified publisher: true
Documentation: charts/valkey-operator/README.md
```

Artifact Hub 측 심사가 끝나기 전까지는, 저장소 코드 변경이나 `gh-pages`
재게시로 `Official` 배지를 `true` 로 전환할 방법이 없다 — 본 단계는
*전적으로 외부 결정 대기* 구간이다.
