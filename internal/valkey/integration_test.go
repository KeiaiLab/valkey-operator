/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:build integration

/*
Copyright 2026 Keiailab.

통합 테스트 — 실제 valkey:8 컨테이너로 cluster bootstrap / replication 알고리즘 검증.
Docker daemon 필요. `make integration-test` 로 실행.

설계 원칙:
  - 외부 의존성 0 (testcontainers-go 등 SDK 미사용, os/exec 만).
  - 각 테스트가 자체 docker network + 컨테이너 정리 (defer cleanup).
  - 빌드 태그 `integration` 으로 일반 `make test` 와 격리.
*/

package valkey

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	valkeyImage    = "valkey/valkey:8"
	containerStart = 5 * time.Second
	clusterReady   = 30 * time.Second
)

// clusterEnv — 통합 테스트가 띄운 valkey cluster 의 네트워크 정보.
//
// macOS Docker Desktop 은 bridge network IP 를 host 에서 라우팅 불가 →
// 노드끼리 통신 (CLUSTER MEET 의 인자 등) 은 internalAddrs (컨테이너 IP) 사용,
// host 의 go-redis client 는 hostAddrs (127.0.0.1:매핑포트) 로 접속.
//
// CLUSTER MEET / ADDSLOTS / REPLICATE / NODES / MYID / FORGET 은 모두 *컨트롤 plane*
// 명령 — MOVED redirect 없음 → host 에서 매핑 포트로 정상 동작.
type clusterEnv struct {
	internalAddrs []string                   // 노드끼리 통신용. CreateCluster 의 addresses 인자.
	hostAddrs     []string                   // host go-redis client 접속용.
	dial          func(string) *redis.Client // internal addr → host addr 매핑 dialer.
	cleanup       func()
}

func startValkeyCluster(t *testing.T, n int) *clusterEnv {
	t.Helper()
	netName := fmt.Sprintf("vk-it-%d", time.Now().UnixNano())
	if out, err := exec.Command("docker", "network", "create", netName).CombinedOutput(); err != nil {
		t.Fatalf("create network: %v: %s", err, out)
	}

	containers := make([]string, 0, n)
	internalAddrs := make([]string, 0, n)
	hostAddrs := make([]string, 0, n)
	mapping := make(map[string]string, n)

	cleanup := func() {
		for _, c := range containers {
			_ = exec.Command("docker", "rm", "-f", c).Run()
		}
		_ = exec.Command("docker", "network", "rm", netName).Run()
	}

	for i := 0; i < n; i++ {
		name := fmt.Sprintf("%s-node-%d", netName, i)
		args := []string{
			"run", "-d", "--rm",
			"--name", name,
			"--network", netName,
			"-p", "6379", // host 동적 포트 매핑 (data plane).
			"-p", "16379", // cluster bus (host 에서 직접 사용 안 하지만 가용성 위해).
			valkeyImage,
			"valkey-server",
			"--cluster-enabled", "yes",
			"--cluster-config-file", "/tmp/nodes.conf",
			"--cluster-node-timeout", "5000",
			"--port", "6379",
			"--appendonly", "no",
			"--bind", "0.0.0.0",
			"--protected-mode", "no",
		}
		if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
			cleanup()
			t.Fatalf("run %s: %v: %s", name, err, out)
		}
		containers = append(containers, name)

		ipOut, err := exec.Command("docker", "inspect", "-f",
			fmt.Sprintf(`{{(index .NetworkSettings.Networks %q).IPAddress}}`, netName), name).Output()
		if err != nil {
			cleanup()
			t.Fatalf("inspect ip %s: %v", name, err)
		}
		ip := strings.TrimSpace(string(ipOut))
		if ip == "" {
			cleanup()
			t.Fatalf("no ip for %s", name)
		}
		internalAddr := fmt.Sprintf("%s:6379", ip)
		internalAddrs = append(internalAddrs, internalAddr)

		// host 매핑 포트 추출.
		portOut, err := exec.Command("docker", "port", name, "6379/tcp").Output()
		if err != nil {
			cleanup()
			t.Fatalf("docker port %s: %v", name, err)
		}
		// 출력 형식: "0.0.0.0:54321\n[::]:54321\n" — 첫 줄 사용.
		firstLine := strings.SplitN(strings.TrimSpace(string(portOut)), "\n", 2)[0]
		_, hostPort, ok := strings.Cut(firstLine, ":")
		if !ok {
			cleanup()
			t.Fatalf("parse host port %q", firstLine)
		}
		hostAddr := "127.0.0.1:" + hostPort
		hostAddrs = append(hostAddrs, hostAddr)
		mapping[internalAddr] = hostAddr
	}

	// 모든 컨테이너가 PONG 응답할 때까지 대기.
	deadline := time.Now().Add(containerStart * 2)
	for _, c := range containers {
		for {
			out, err := exec.Command("docker", "exec", c, "valkey-cli", "ping").Output()
			if err == nil && strings.TrimSpace(string(out)) == "PONG" {
				break
			}
			if time.Now().After(deadline) {
				cleanup()
				t.Fatalf("%s never became ready", c)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	dial := func(internalAddr string) *redis.Client {
		hostAddr, ok := mapping[internalAddr]
		if !ok {
			hostAddr = internalAddr // fallback (Linux host networking 등).
		}
		return NewSingleClient(DialOptions{Address: hostAddr})
	}

	return &clusterEnv{
		internalAddrs: internalAddrs,
		hostAddrs:     hostAddrs,
		dial:          dial,
		cleanup:       cleanup,
	}
}

// 시나리오 1: 6 노드 → CreateCluster → state ok → 멱등 재호출 안전.
func TestIntegration_CreateCluster_idempotent(t *testing.T) {
	env := startValkeyCluster(t, 6)
	defer env.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), clusterReady)
	defer cancel()

	if err := CreateCluster(ctx, env.dial, env.internalAddrs, 3, 1); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	// gossip 수렴 대기 + 검증.
	if !waitClusterOK(t, ctx, env.dial(env.internalAddrs[0]), 3, 30*time.Second) {
		t.Fatal("cluster never became ok")
	}

	// 멱등 재호출 — 이미 부트스트랩된 클러스터에 다시 CreateCluster (각 단계가 skip).
	if err := CreateCluster(ctx, env.dial, env.internalAddrs, 3, 1); err != nil {
		t.Fatalf("CreateCluster (idempotent re-run): %v", err)
	}

	// CLUSTER NODES 검증 — 6 노드 + 3 primary + 3 replica.
	c := env.dial(env.internalAddrs[0])
	defer func() { _ = c.Close() }()
	nodes, err := QueryClusterNodes(ctx, c)
	if err != nil {
		t.Fatalf("QueryClusterNodes: %v", err)
	}
	if len(nodes) != 6 {
		t.Errorf("nodes len: %d", len(nodes))
	}
	primaries, replicas := 0, 0
	for _, n := range nodes {
		switch {
		case n.IsPrimary():
			primaries++
		case n.IsReplica():
			replicas++
		}
	}
	if primaries != 3 || replicas != 3 {
		t.Errorf("topology: primaries=%d replicas=%d (want 3/3)", primaries, replicas)
	}
}

// 시나리오 2: ForgetNode — scale-in 후 잔존 primary 들에서 forget.
func TestIntegration_ForgetNode(t *testing.T) {
	env := startValkeyCluster(t, 6)
	defer env.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), clusterReady)
	defer cancel()

	if err := CreateCluster(ctx, env.dial, env.internalAddrs, 3, 1); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	waitClusterOK(t, ctx, env.dial(env.internalAddrs[0]), 3, 30*time.Second)

	// 마지막 replica 의 node id 추출.
	c := env.dial(env.internalAddrs[5])
	id, err := c.ClusterMyID(ctx).Result()
	_ = c.Close()
	if err != nil {
		t.Fatalf("get last replica id: %v", err)
	}

	// 모든 잔존 노드에서 forget. best-effort (이미 모르는 id 는 무시).
	for _, addr := range env.internalAddrs[:5] {
		c := env.dial(addr)
		err := ForgetNode(ctx, c, id)
		_ = c.Close()
		if err != nil && !strings.Contains(err.Error(), "Unknown node") {
			t.Logf("forget %s: %v (ignored)", addr, err)
		}
	}
}

// (EnsureReplicaOf / PromoteToPrimary 는 단일 Valkey 모드 (cluster-enabled no) 영역.
// 본 통합 테스트 모듈은 cluster 부트스트랩 / 멱등성 / forget 에 집중.
// standalone replication 통합 테스트는 ValkeyReconciler 영역으로 별도 PR.)

// 시나리오 4: 부트스트랩 후 CLUSTER NODES 응답이 정확히 3 primary + 3 replica 토폴로지를
// 보고하며 각 primary 의 slot range 가 비중복 + 합 16384 인지 검증.
func TestIntegration_NodesTopology(t *testing.T) {
	env := startValkeyCluster(t, 6)
	defer env.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), clusterReady)
	defer cancel()

	if err := CreateCluster(ctx, env.dial, env.internalAddrs, 3, 1); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if !waitClusterOK(t, ctx, env.dial(env.internalAddrs[0]), 3, 30*time.Second) {
		t.Fatal("cluster never became ok")
	}

	c := env.dial(env.internalAddrs[0])
	defer func() { _ = c.Close() }()
	nodes, err := QueryClusterNodes(ctx, c)
	if err != nil {
		t.Fatalf("QueryClusterNodes: %v", err)
	}

	// primary 노드들의 slot 합산 = 16384 + 비중복.
	var totalSlots int32
	covered := make(map[int]bool, 16384)
	primaryCount := 0
	for _, n := range nodes {
		if !n.IsPrimary() {
			continue
		}
		primaryCount++
		for _, r := range n.Slots {
			for s := r.Start; s <= r.End; s++ {
				if covered[s] {
					t.Errorf("slot %d assigned to multiple primaries", s)
				}
				covered[s] = true
				totalSlots++
			}
		}
	}
	if primaryCount != 3 {
		t.Errorf("primary count: got %d want 3", primaryCount)
	}
	if totalSlots != 16384 {
		t.Errorf("total slots covered: got %d want 16384", totalSlots)
	}

	// replica 노드들의 MasterID 가 모두 primary 의 ID 와 일치해야 함.
	primaryIDs := make(map[string]bool)
	for _, n := range nodes {
		if n.IsPrimary() {
			primaryIDs[n.ID] = true
		}
	}
	for _, n := range nodes {
		if n.IsReplica() && !primaryIDs[n.MasterID] {
			t.Errorf("replica %s has unknown MasterID %s", n.Addr, n.MasterID)
		}
	}
}

// 시나리오 3: ensureMeet 의 partial 회복 — 이미 일부 노드끼리 만난 상태에서 추가 노드 합류.
func TestIntegration_EnsureMeet_partial(t *testing.T) {
	env := startValkeyCluster(t, 4)
	defer env.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), clusterReady)
	defer cancel()

	// 처음 2 노드만 먼저 MEET (수동).
	if err := ensureMeet(ctx, env.dial, env.internalAddrs[:2]); err != nil {
		t.Fatalf("partial meet (2 nodes): %v", err)
	}

	// 나머지 2 노드 포함하여 다시 MEET — 멱등 + partial 회복.
	if err := ensureMeet(ctx, env.dial, env.internalAddrs); err != nil {
		t.Fatalf("ensureMeet (4 nodes): %v", err)
	}

	// 첫 노드의 NODES 응답에 4 멤버 모두 보임.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		c := env.dial(env.internalAddrs[0])
		nodes, err := QueryClusterNodes(ctx, c)
		_ = c.Close()
		if err == nil && len(nodes) >= 4 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("4 nodes never converged")
}

// waitClusterOK — IsClusterReady poll. 성공 시 true.
func waitClusterOK(t *testing.T, ctx context.Context, c *redis.Client, expectedSize int32, timeout time.Duration) bool {
	t.Helper()
	defer func() { _ = c.Close() }()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready, _, err := IsClusterReady(ctx, c, expectedSize); err == nil && ready {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
