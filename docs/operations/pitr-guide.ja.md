# Point-In-Time Recovery (PITR) — operator ガイド (日本語)

> English: [pitr-guide.md](pitr-guide.md) — canonical / 正本

PITR は ADR-0040 商用 parity チェックリストにおける *最大の単一ギャップ* であった。
本書では **phase 1** ガイド (API + webhook) に加え、**phase 2** (reconciler dispatch) への
進入経路までを扱う。

## 現状 (2026-05-10 時点)

| 領域 | 状態 |
|---|---|
| AOF backup (生成) | ✅ GA (BgRewriteAOF、ADR-0016 + minio-go / GCS / Azure) |
| RDB backup (生成) | ✅ GA |
| `ValkeyRestore.Spec.PointInTime` API | ✅ GA (#54) |
| Webhook validation (Source 3 種 + PointInTime+RDB の reject) | ✅ GA (#54) |
| **AOF replay-to-timestamp reconciler dispatch** | ❌ phase 2 |
| 手動 PITR (operator 外部ツール) | ✅ 利用可能 |

## Phase 1 の利用法 — AOF 全 replay (`PointInTime` nil)

最も一般的なケースで、backup の全データを復元する。動作は #54 以前と同等:

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata:
  name: vk-restore-full
  namespace: valkey
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF   # backup も AOF で取得済みの場合
```

reconciler が AOF を download → init container が Valkey データディレクトリへ配置 →
STS を再起動 → Valkey が起動時に AOF 全体を replay する。

## Phase 1 の利用法 — PITR API (PointInTime あり、dispatch 未実装)

webhook は spec を受理し、`status` も保持される。reconciler は現状「AOF 全 replay」と
同じ動作になる (PointInTime は無視) — phase 2 までの **fail-safe** 動作:

```yaml
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef:
      name: s3-prod
      path: vk-prod/2026-05-10T00:00:00Z/dump.aof
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"   # 目標復元時刻
```

**現状**: webhook が invariants を検証する (RDB + PointInTime は reject)。reconciler は
`PointInTime` を無視して AOF 全体を replay するため、より早い時点への切り戻しが
必要なら **短い** AOF を渡す。**Phase 2 (別 epic)** でこの正確な時刻で truncate する
dispatch を追加する。

## 手動 PITR (phase 2 の代替手段)

phase 2 が入るまでの暫定運用手順。operator 外のツールを利用する:

1. **AOF を download する**:
   ```sh
   aws s3 cp s3://vk-prod-backups/2026-05-10T00:00:00Z/dump.aof ./dump.aof
   ```

2. **AOF を truncate する** — 目標時刻までの entry のみを残す
   (Valkey AOF format を直接編集):
   ```sh
   # AOF entry から timestamp を抽出する (TIMESTAMP-aware AOF のみ — Valkey 8.0+ で
   # `set aof-timestamp-enabled yes` が必要)。
   valkey-aof-trim --until "2026-05-10T14:30:00Z" dump.aof > dump-truncated.aof
   ```
   **注**: `valkey-aof-trim` は外部ツール / ユーザー自作のスクリプトである。Valkey
   公式の utility は 9.x で追加予定。

3. **truncate 済み AOF を upload する**:
   ```sh
   aws s3 cp dump-truncated.aof s3://vk-prod-backups/pitr-2026-05-10T14:30:00Z/dump.aof
   ```

4. **truncate 済み AOF で復元する**:
   ```yaml
   spec:
     source:
       targetRef:
         name: s3-prod
         path: pitr-2026-05-10T14:30:00Z/dump.aof
     restoreType: AOF
   ```

phase 2 では 1〜3 のステップが operator 内部で **自動化** される。

## Phase 2 進入条件 (別 epic 候補)

本ガイドの dispatch を有効化するために必要な要件:

1. ~~**AOF-timestamp parse ライブラリ**~~ → ✅ **#68** `internal/aoftime` パッケージ GA。
2. ~~**reconciler 統合向け file-level helper**~~ → ✅ **#69** `TruncateAOFFile` GA。
3. ~~**Reconciler dispatch — download Job 内の cli が in-place で truncate**~~ → ✅ **#70** (`DownloadJobParams.PITRCutoff` + `cli download --pitr-cutoff`; `PointInTime` 指定かつ `RestoreType=AOF` のとき reconciler が自動的に dispatch する)。
4. **`valkey-cli --pipe` 統合** — 現状は init container が起動時に AOF を load する
   (Valkey の default `appendonly yes` 挙動)。`valkey-cli --pipe` の個別統合は
   **streaming replay** が必要なシナリオに限り意味を持つ。現状の init container
   経路で十分。
5. **`PointInTime ≤ backup CompletedAt` の webhook invariant** (follow-up) — backup
   完了より後の `PointInTime` は意味的に矛盾している (まだ存在しないデータを要求
   している)。
6. **rollback** (follow-up) — replay 失敗時に backup 時点へ fallback する。

**現状 (#70 後)**: `restoreType: AOF` + `PointInTime` の場合、reconciler が自動的に
download → truncate を行い、init container が truncate 済みの AOF から起動する。
**完全自動 PITR** は稼働可能である。残作業は webhook invariant と rollback (運用安全性)。

## 失敗した PITR からの復旧 (#72 rollback)

PITR replay が失敗した場合 (AOF 破損、timestamp marker 不正など)、init container が
CrashLoopBackOff に陥る。手動 rollback:

### 前提条件

reconciler は download Job を `--pitr-backup=/backup/dump.aof.original` 付きで起動する
(#72)。当該 backup ファイルが staging PVC 上に存在している必要がある。

### 自動 rollback (運用者の 1-liner)

```sh
# 1. staging PVC にアクセス可能な helper pod を起動する。
kubectl run rollback-helper --rm -it --restart=Never \
  --image=ghcr.io/keiailab/valkey-operator:latest \
  --overrides='{"spec":{"containers":[{"name":"r","image":"ghcr.io/keiailab/valkey-operator:latest","command":["sh","-c","cp /backup/dump.aof.original /backup/dump.aof"],"volumeMounts":[{"name":"b","mountPath":"/backup"}]}],"volumes":[{"name":"b","persistentVolumeClaim":{"claimName":"<staging-pvc>"}}]}}'

# 2. Valkey STS を再起動する (init container が AOF 全体を replay する)。
kubectl rollout restart sts/<cluster-name>
```

### 自動化 (operator 側、別 epic)

follow-up — operator が以下を担当する:

1. `Status.Phase=Restoring` + init container の CrashLoopBackOff を検知する。
2. backup ファイルの存在を検証する。
3. `Status.Phase=PITRRollbackPending` へ遷移する (利用者の明示承認後に自動)。
4. 上記 1-liner を reconciler から実行する。

本自動化は **destructive** (PVC データを上書きする) であるため、ADR とユーザーの
明示承認の双方を要件とする。

## #70 利用例 (実動作)

```yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyRestore
metadata: { name: pitr-restore }
spec:
  clusterRef: { kind: Valkey, name: vk-prod }
  source:
    targetRef: { name: s3-prod, path: backup/dump.aof }
  restoreType: AOF
  pointInTime: "2026-05-10T14:30:00Z"
```

内部フロー:

1. `handlePending`: webhook (#54) が invariants を検証 → Mounting。
2. `handleMounting`: download Job を `--pitr-cutoff=2026-05-10T14:30:00Z` 付きで作成。
3. `cli download` (#70): S3 → `/backup/dump.aof` → cutoff まで in-place truncate。
4. `handleRestoring`: 既存の init container 経路 → cluster が truncate 済み AOF
   から起動。
5. Verifying → Completed。

## #68 利用例 (Go コードへの統合)

```go
import "github.com/keiailab/valkey-operator/internal/aoftime"

aofBytes, _ := os.ReadFile("dump.aof")
if !aoftime.HasTimestamps(aofBytes) {
    // PITR 不可 — 全 replay のみ可能。
    return errors.New("AOF lacks timestamps (set aof-timestamp-enabled yes for PITR)")
}
cutoff := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
offset := aoftime.TruncateOffset(aofBytes, cutoff)
truncated := aofBytes[:offset]
// `truncated` を `valkey-cli --pipe` に stream する → cutoff までの entry のみが
// 復元される。
```

## 関連

- runbook §3.3 — Restore (災害復旧)。
- ADR-0015 — `ValkeyRestore` の init container パターン。
- ADR-0016 — `ValkeyBackupTarget` 外部 storage。
- #54 — `PointInTime` API + webhook。

## 参照

- Valkey AOF spec: <https://valkey.io/topics/persistence/>
- AOF timestamp-enabled (8.0+): `aof-timestamp-enabled` directive。
- 外部ツール: `redis-cli --pipe` (Valkey 互換)。
