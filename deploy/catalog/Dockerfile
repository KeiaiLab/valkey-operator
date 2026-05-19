# File-Based Catalog image for valkey-operator.
#
# Refs:
# - ADR-0137 (infra/bootstrap): valkey-operator OLM v1 ClusterExtension Migration
# - mongodb-operator/deploy/catalog/Dockerfile (정합 패턴)
#
# OLM CatalogSource (image: ghcr.io/keiailab/valkey-operator-catalog:vX.Y.Z) 가
# 본 image 를 pull. gRPC server `opm serve /configs` 가 OLM resolver 에 stream.
FROM quay.io/operator-framework/opm:v1.42.0

# OLM 1.27+ FBC 표준 — /configs 디렉토리 의 YAML/JSON 을 자동 인식.
LABEL operators.operatorframework.io.index.configs.v1=/configs

ADD catalog /configs

# Pre-cache 는 minimal FBC (olm.bundle.object 미포함) 와 충돌 — runtime cache
# 로 대체. OLM resolver 가 첫 query 시 cache 자동 생성.
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs"]
