# Artifact Hub 신뢰 배지 운영 절차

본 문서는 `valkey-operator` Artifact Hub 패키지의 `Signed` / `Official`
상태를 운영 가능한 절차로 고정한다.

## 현재 상태 판정

2026-05-12 확인 기준:

```bash
curl -fsSL https://artifacthub.io/api/v1/packages/helm/keiailab-valkey-operator/valkey-operator \
  | jq '{version, signed, official, repository:{verified_publisher:.repository.verified_publisher, official:.repository.official}}'
```

관찰값:

- `repository.verified_publisher=true`
- `signed=false`
- `repository.official=false`
- `https://keiailab.github.io/valkey-operator/valkey-operator-1.0.10.tgz.prov` 는 404

즉 repository ownership 검증은 끝났고, 누락된 것은 Helm provenance 와
Artifact Hub official 심사다.

## Signed 적용

Artifact Hub 의 Helm `Signed` 표시는 chart archive 와 같은 경로에
`<chart>-<version>.tgz.prov` 가 있어야 한다. Helm provenance 는
`helm package --sign` 시점에 생성된다.

릴리스 기본값:

```bash
make release VERSION=vX.Y.Z
```

`Makefile` 기본값이 `HELM_SIGN=1` 이므로, 위 명령은 다음 전제조건이 없으면
실패한다.

- `~/.gnupg/secring.gpg` 존재
- `HELM_GPG_KEY=Keiailab Helm`
- `HELM_GPG_FINGERPRINT=89A409476828CB992338C378651E51AF520BCB78`

secret key 를 새 개발 장비에 준비하는 절차:

```bash
gpg --import <keiailab-helm-private-key.asc>
gpg --export-secret-keys > ~/.gnupg/secring.gpg
make helm-signing-preflight
```

릴리스 후 검증:

```bash
bash scripts/release-smoke-test.sh vX.Y.Z
```

이 스모크는 GH Release 의 `.tgz.prov`, gh-pages 의 `.tgz.prov`, 그리고
`artifacthub-repo.yml` 의 public signing key 로 `helm verify` 를 수행한다.

## Official 적용

`Official` 은 저장소 파일만으로 켤 수 있는 값이 아니다. Artifact Hub 기준상
퍼블리셔가 패키지의 주 소프트웨어를 소유한다는 외부 심사 상태다.

현재 충족된 선행조건:

- Artifact Hub repository ID:
  `16085dd0-0f19-4c6b-ab90-bd97105bdf42`
- Verified publisher: `true`
- Chart README: `charts/valkey-operator/README.md`
- Repository metadata served:
  `https://keiailab.github.io/valkey-operator/artifacthub-repo.yml`

남은 조치:

1. Artifact Hub 에 로그인한 publisher 또는 organization member 가
   `artifacthub/hub` 의 official status request issue 를 생성한다.
2. repository-level official 을 요청한다.
3. 요청 내용에 다음 사실을 포함한다.

```text
Repository name: keiailab-valkey-operator
Package name: valkey-operator
Repository URL: https://keiailab.github.io/valkey-operator
Package URL: https://artifacthub.io/packages/helm/keiailab-valkey-operator/valkey-operator
Publisher owns the software: yes, keiailab publishes and maintains valkey-operator itself.
Verified publisher: true
Documentation: charts/valkey-operator/README.md
```

Artifact Hub 심사 완료 전까지는 로컬 코드 변경이나 gh-pages publish 만으로
`Official` badge 가 `true` 로 바뀌지 않는다.
