# Credential sourcing with External Secrets

Valkey runtime credentials remain ordinary Kubernetes `Secret` references in
the CRD API. Production clusters should materialize those Secrets through
External Secrets Operator, backed by Infisical or another approved secret
store.

## Helm values

```yaml
externalSecrets:
  enabled: true
  secretStoreKind: ClusterSecretStore
  secretStoreName: infisical
  refreshInterval: 1h
  auth:
    enabled: true
    name: valkey-auth
    targetName: valkey-auth
    passwordKey: password
    remoteKey: /data/valkey/auth/password
```

Then reference the materialized Secret from the Valkey CR:

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cache
spec:
  auth:
    enabled: true
    passwordSecretRef:
      name: valkey-auth
      key: password
```

## Backup target credentials

```yaml
externalSecrets:
  enabled: true
  backupTarget:
    enabled: true
    name: valkey-backup-s3
    targetName: valkey-backup-s3
    accessKeyIdRemoteKey: /data/valkey/backup/access-key-id
    secretAccessKeyRemoteKey: /data/valkey/backup/secret-access-key
```

Reference `valkey-backup-s3` from the `ValkeyBackupTarget` credentials
field. Do not commit a raw Kubernetes `Secret` manifest with S3 keys.

## Verification

```sh
kubectl get externalsecret -A | grep valkey
kubectl get secret valkey-auth -o jsonpath='{.data.password}' | wc -c
kubectl describe valkey cache | grep -E 'Auth|Ready|Phase'
```

If `ExternalSecret` is ready but Valkey auth fails, compare only Secret key
names and references. Do not print decoded secret values into terminal logs.
