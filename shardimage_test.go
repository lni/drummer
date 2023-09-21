// Copyright 2017-2019 Lei Ni (nilei81@gmail.com) and other Dragonboat authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !dragonboat_monkeytest
// +build !dragonboat_monkeytest

package drummer

import (
	"reflect"
	"testing"

	pb "github.com/lni/drummer/v3/drummerpb"
)

func TestZeroTickZeroFirstObservedNodeIsConsideredAsDailed(t *testing.T) {
	n := replica{
		Tick:          0,
		FirstObserved: 0,
	}
	if !n.failed(1) {
		t.Errorf("node is not reported as failed")
	}
	n.Tick = 1
	if n.failed(1) {
		t.Errorf("node is not expected to be considered as failed")
	}
	n.Tick = 0
	n.FirstObserved = 1
	if n.failed(1) {
		t.Errorf("node is not expected to be considered as failed")
	}
}

func TestZombieNodeWithoutClustedrInfoWillBeReported(t *testing.T) {
	mc := newMultiShard()
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 100,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
	}
	mc.update(nhi)

	ci2 := &pb.ShardInfo{
		ShardId:   1,
		ReplicaId: 4,
		Pending:   true,
	}
	nhi2 := pb.NodeHostInfo{
		RaftAddress: "a4",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci2},
	}
	mc.update(nhi2)

	if len(mc.ReplicasToKill) != 1 {
		t.Fatalf("replicas to kill not updated")
	}
	ntk := mc.ReplicasToKill[0]
	if ntk.ShardID != 1 {
		t.Errorf("shard id %d, want 1", ntk.ShardID)
	}
	if ntk.ReplicaID != 4 {
		t.Errorf("node id %d, want 4", ntk.ReplicaID)
	}
	if ntk.Address != "a4" {
		t.Errorf("address %s, want a4", ntk.Address)
	}
}

func TestZombieNodeWillBeReported(t *testing.T) {
	mc := newMultiShard()
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 100,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
	}
	mc.update(nhi)

	ci2 := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         4,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3", 4: "a4"},
		ConfigChangeIndex: 50,
	}
	nhi2 := pb.NodeHostInfo{
		RaftAddress: "a4",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci2},
	}
	mc.update(nhi2)

	if len(mc.ReplicasToKill) != 1 {
		t.Fatalf("replicas to kill not updated")
	}
	ntk := mc.ReplicasToKill[0]
	if ntk.ShardID != 1 {
		t.Errorf("shard id %d, want 1", ntk.ShardID)
	}
	if ntk.ReplicaID != 4 {
		t.Errorf("node id %d, want 4", ntk.ReplicaID)
	}
	if ntk.Address != "a4" {
		t.Errorf("address %s, want a4", ntk.Address)
	}
	gntk := mc.getToKillReplicas()
	if len(mc.ReplicasToKill) != 0 {
		t.Errorf("failed to clear the ntk list")
	}
	if len(gntk) != 1 {
		t.Errorf("failed to get the ntk list")
	}
	grntk := gntk[0]
	if grntk.ShardID != ntk.ShardID ||
		grntk.ReplicaID != ntk.ReplicaID ||
		grntk.Address != ntk.Address {
		t.Errorf("unexpected value")
	}
}

func TestMultiShardUpdateCanAddShard(t *testing.T) {
	mc := newMultiShard()
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 1,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
	}
	mc.update(nhi)

	if len(mc.Shards) != 1 {
		t.Errorf("sz = %d, want %d", len(mc.Shards), 1)
	}

	v, ok := mc.Shards[1]
	if !ok {
		t.Error("shard id suppose to be 1")
	}
	if v.ShardID != 1 {
		t.Errorf("shard id %d, want 1", v.ShardID)
	}
	if v.ConfigChangeIndex != 1 {
		t.Errorf("ConfigChangeIndex = %d, want 1", v.ConfigChangeIndex)
	}
	if len(v.Replicas) != 3 {
		t.Errorf("replicas sz = %d, want 3", len(v.Replicas))
	}
	if v.Replicas[2].Tick != 100 {
		t.Errorf("tick = %d, want 100", v.Replicas[2].Tick)
	}
	if v.Replicas[1].Tick != 0 {
		t.Errorf("tick = %d, want 0", v.Replicas[1].Tick)
	}
	if v.Replicas[1].FirstObserved != 100 {
		t.Errorf("first observed = %d, want 100", v.Replicas[1].FirstObserved)
	}
	if v.Replicas[2].Address != "a2" {
		t.Errorf("address = %s, want a2", v.Replicas[2].Address)
	}
}

func TestMultiShardUpdateCanUpdateShard(t *testing.T) {
	mc := newMultiShard()
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 1,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
	}
	mc.update(nhi)

	// higher ConfigChangeIndex will be accepted
	uci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          false,
		Replicas:          map[uint64]string{2: "a2", 3: "a3", 4: "a4", 5: "a5"},
		ConfigChangeIndex: 2,
	}
	unhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    200,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{uci},
	}
	mc.update(unhi)
	v := mc.Shards[1]
	if v.ConfigChangeIndex != 2 {
		t.Errorf("ConfigChangeIndex = %d, want 2", v.ConfigChangeIndex)
	}
	if len(v.Replicas) != 4 {
		t.Errorf("replicas sz = %d, want 4", len(v.Replicas))
	}
	// node 1 expected to be gone
	hasNode1 := false
	for _, n := range v.Replicas {
		if n.ReplicaID == 1 {
			hasNode1 = true
		}
	}
	if hasNode1 {
		t.Error("node 1 is not deleted")
	}

	if v.Replicas[2].Tick != 200 {
		t.Errorf("tick = %d, want 200", v.Replicas[2].Tick)
	}

	// lower ConfigChangeIndex will be ignored
	uci = &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          false,
		Replicas:          map[uint64]string{1: "a1", 2: "a2"},
		ConfigChangeIndex: 1,
	}
	unhi = pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    200,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{uci},
	}
	mc.update(unhi)
	if v.ConfigChangeIndex != 2 {
		t.Errorf("ConfigChangeIndex = %d, want 2", v.ConfigChangeIndex)
	}
	if len(v.Replicas) != 4 {
		t.Errorf("replicas sz = %d, want 4", len(v.Replicas))
	}
}

func testMultiShardTickRegionUpdate(t *testing.T,
	pending bool, incomplete bool) {
	mc := newMultiShard()
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 1,
		Pending:           false,
		Incomplete:        false,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
	}
	mc.update(nhi)

	if mc.Shards[1].Replicas[2].Tick != 100 {
		t.Errorf("tick not set")
	}
	nhi.ShardInfo[0].Pending = pending
	nhi.ShardInfo[0].Incomplete = incomplete
	nhi.LastTick = 200
	nhi.Region = "region-new"
	mc.update(nhi)
	if mc.Shards[1].Replicas[2].Tick != 200 {
		t.Errorf("tick not updated")
	}
}

func TestMultiShardTickRegionUpdate(t *testing.T) {
	testMultiShardTickRegionUpdate(t, false, false)
	testMultiShardTickRegionUpdate(t, false, true)
	testMultiShardTickRegionUpdate(t, true, false)
}

func TestMultiShardDeepCopy(t *testing.T) {
	mc := newMultiShard()
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 1,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
	}
	mc.update(nhi)
	dcmc := mc.deepCopy()

	mc.Shards[1].Replicas[2].IsLeader = false
	if !dcmc.Shards[1].Replicas[2].IsLeader {
		t.Errorf("is leader = %t, want true", dcmc.Shards[1].Replicas[2].IsLeader)
	}

	// restore the value
	mc.Shards[1].Replicas[2].IsLeader = true
	if !reflect.DeepEqual(mc, dcmc) {
		t.Error("Deep copied MultiShard is not reflect.DeepEqual")
	}
}

func TestNodeFailed(t *testing.T) {
	expected := []struct {
		nodeTick          uint64
		nodeFirstObserved uint64
		tick              uint64
		failed            bool
	}{
		{1, 1, 1 + nodeHostTTL, false},
		{1, 1, 2 + nodeHostTTL, true},
	}

	for idx, v := range expected {
		n := replica{
			Tick:          v.nodeTick,
			FirstObserved: v.nodeFirstObserved,
		}
		f := n.failed(v.tick)
		if f != v.failed {
			t.Errorf("%d, failed %t, want %t", idx, f, v.failed)
		}
	}
}

func TestNodeWaitingToBeStarted(t *testing.T) {
	expected := []struct {
		nodeTick          uint64
		nodeFirstObserved uint64
		tick              uint64
		wst               bool
	}{
		{0, 1, 1 + 10*nodeHostTTL, true},
		{1, 1, 1 + nodeHostTTL, false},
		{1, 1, nodeHostTTL, false},
	}

	for idx, v := range expected {
		n := replica{
			Tick:          v.nodeTick,
			FirstObserved: v.nodeFirstObserved,
		}
		w := n.waitingToBeStarted(v.tick)
		if w != v.wst {
			t.Errorf("%d, wst %t, want %t", idx, w, v.wst)
		}
	}
}

func TestShardReplicasStatus(t *testing.T) {
	base := uint64(100000)
	expected := []struct {
		nodeTick          uint64
		nodeFirstObserved uint64
		failed            bool
		wst               bool
	}{
		{0, base - nodeHostTTL, false, true},
		{0, base - nodeHostTTL - 1, false, true},
		{base - nodeHostTTL - 1, 0, true, false},
		{base - nodeHostTTL, 0, false, false},
	}

	c := &shard{}
	c.Replicas = make(map[uint64]*replica)
	for idx, v := range expected {
		n := &replica{
			Tick:          v.nodeTick,
			FirstObserved: v.nodeFirstObserved,
		}

		if n.failed(base) != v.failed {
			t.Errorf("%d, failed %t, want %t", idx, n.failed(base), v.failed)
		}
		if n.waitingToBeStarted(base) != v.wst {
			t.Errorf("%d, wst %t, want %t", idx, n.waitingToBeStarted(base), v.wst)
		}

		c.Replicas[uint64(idx)] = n
	}

	if c.quorum() != 3 {
		t.Errorf("quorum = %d, want 3", c.quorum())
	}
	if len(c.getOkReplicas(base)) != 1 {
		t.Errorf("oknode sz = %d, want 1", len(c.getOkReplicas(base)))
	}
	if len(c.getFailedReplicas(base)) != 1 {
		t.Errorf("failed replicas sz = %d, want 1", len(c.getFailedReplicas(base)))
	}
	if len(c.getReplicasToStart(base)) != 2 {
		t.Errorf("replicas to start sz = %d, want 2", len(c.getReplicasToStart(base)))
	}
	if c.available(base) {
		t.Errorf("available = %t, want false", c.available(base))
	}
}

func TestShardReqair(t *testing.T) {
	base := uint64(100000)
	expected := []struct {
		nodeTick          uint64
		nodeFirstObserved uint64
		failed            bool
		wst               bool
	}{
		{0, base - nodeHostTTL, false, true},
		{0, base - nodeHostTTL - 1, false, true},
		{base - nodeHostTTL - 1, 0, true, false},
		{base - nodeHostTTL, 0, false, false},
	}

	c := &shard{}
	c.Replicas = make(map[uint64]*replica)
	for idx, v := range expected {
		n := &replica{
			Tick:          v.nodeTick,
			FirstObserved: v.nodeFirstObserved,
		}

		if n.failed(base) != v.failed {
			t.Errorf("%d, failed %t, want %t", idx, n.failed(base), v.failed)
		}
		if n.waitingToBeStarted(base) != v.wst {
			t.Errorf("%d, wst %t, want %t", idx, n.waitingToBeStarted(base), v.wst)
		}

		c.Replicas[uint64(idx)] = n
	}

	mc := &multiShard{}
	mc.Shards = make(map[uint64]*shard)
	mc.Shards[1] = c
	crl := mc.getShardForRepair(base)
	if len(crl) != 1 {
		t.Errorf("shard repair sz:%d, want 1", len(crl))
	}

	if crl[0].shard != c {
		t.Errorf("shard not pointing to correct struct")
	}

	if len(crl[0].failedReplicas) != 1 {
		t.Errorf("failed replicas sz = %d, want 1", len(crl[0].failedReplicas))
	}
	if len(crl[0].okReplicas) != 1 {
		t.Errorf("ok replicas sz = %d, want 1", len(crl[0].okReplicas))
	}
	if len(crl[0].replicasToStart) != 2 {
		t.Errorf("wst replicas sz = %d, want 2", len(crl[0].replicasToStart))
	}

	cr := crl[0]
	if cr.quorum() != 3 {
		t.Errorf("quorum = %d, want 3", cr.quorum())
	}
	if cr.available() {
		t.Errorf("available = %t, want false", cr.available())
	}
	if cr.addRequired() {
		t.Errorf("add required = %t, want false", cr.addRequired())
	}
	if !cr.createRequired() {
		t.Errorf("create required = %t, want true", cr.createRequired())
	}
	if !cr.hasReplicasToStart() {
		t.Errorf("has node to start = %t, want true", cr.hasReplicasToStart())
	}
}
