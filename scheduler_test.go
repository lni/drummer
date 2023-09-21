// Copyright 2017-2019 Lei Ni (nilei81@gmail.com).
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
	"math/rand"
	"reflect"
	"testing"

	pb "github.com/lni/drummer/v3/drummerpb"
	"github.com/lni/drummer/v3/settings"
	"github.com/lni/goutils/random"
	"google.golang.org/protobuf/proto"
)

func getShard() []*pb.Shard {
	c := &pb.Shard{
		Members: []uint64{1, 2, 3},
		ShardId: 100,
		AppName: "noop",
	}
	return []*pb.Shard{c}
}

type testNodeInfo struct {
	address     string
	tick        uint64
	region      string
	hasShard    bool
	shardID     uint64
	replicaID   uint64
	allReplicas []uint64
	addressList []string
}

func getTestNodeHostInfo(nhi []testNodeInfo) []pb.NodeHostInfo {
	result := make([]pb.NodeHostInfo, 0)
	for _, n := range nhi {
		nh := pb.NodeHostInfo{
			RaftAddress: n.address,
			LastTick:    n.tick,
			Region:      n.region,
		}
		if n.hasShard {
			ci := &pb.ShardInfo{
				ShardId:   n.shardID,
				ReplicaId: n.replicaID,
			}
			nodeMap := make(map[uint64]string)
			for idx, nid := range n.allReplicas {
				nodeMap[nid] = n.addressList[idx]
			}
			ci.Replicas = nodeMap
			nh.ShardInfo = make([]*pb.ShardInfo, 0)
			nh.ShardInfo = append(nh.ShardInfo, ci)
		}
		result = append(result, nh)
	}
	return result
}

func getTestMultiShard(nhList []pb.NodeHostInfo) *multiShard {
	mc := newMultiShard()
	for _, nh := range nhList {
		mc.update(nh)
	}
	return mc
}

func getTestMultiNodeHost(nhList []pb.NodeHostInfo) *multiNodeHost {
	mnh := newMultiNodeHost()
	for _, nh := range nhList {
		mnh.update(nh)
	}
	return mnh
}

// without any running cluster
func getNodeHostInfoListWithoutAnyShard() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 100, "region-1", false, 0, 0, nil, nil},
		{"a2", 100, "region-2", false, 0, 0, nil, nil},
		{"a3", 100, "region-3", false, 0, 0, nil, nil},
		{"a4", 100, "region-4", false, 0, 0, nil, nil},
	}
	return getTestNodeHostInfo(nhi)
}

//
// with one just restarted node, no node on that restarted node
//

func getNodeHostInfoListWithOneRestartedNode() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 500, "region-1", true, 100, 1, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a2", 500, "region-2", true, 100, 2, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a3", 500, "region-3", false, 0, 0, nil, nil},
		{"a4", 500, "region-4", false, 0, 0, nil, nil},
	}
	nh := getTestNodeHostInfo(nhi)
	nh[2].PlogInfoIncluded = true
	nh[2].PlogInfo = append(nh[2].PlogInfo, &pb.LogInfo{ShardId: 100, ReplicaId: 3})
	return nh
}

//
// with one failed node and there was cluster on that node.
//

func getNodeHostInfoListWithOneFailedShard() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 500, "region-1", true, 100, 1, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a2", 500, "region-2", true, 100, 2, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a3", 100, "region-3", true, 100, 3, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a4", 500, "region-4", false, 0, 0, nil, nil},
	}
	return getTestNodeHostInfo(nhi)
}

//
// with one added node
//

func getNodeHostInfoListWithOneAddedShard() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 500, "region-1", true, 100, 1, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
		{"a2", 500, "region-2", true, 100, 2, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
		{"a3", 100, "region-3", true, 100, 3, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
		{"a4", 500, "region-4", false, 0, 0, nil, nil},
	}
	return getTestNodeHostInfo(nhi)
}

//
// with one ready to be deleted
//

func getNodeHostInfoListWithOneReadyForDeleteShard() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 500, "region-1", true, 100, 1, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
		{"a2", 500, "region-2", true, 100, 2, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
		{"a3", 100, "region-3", true, 100, 3, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
		{"a4", 500, "region-4", true, 100, 4, []uint64{1, 2, 3, 4}, []string{"a1", "a2", "a3", "a4"}},
	}
	return getTestNodeHostInfo(nhi)
}

//
// Restore cluster
//

func getNodeHostListWithUnavailableShard() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 500, "region-1", true, 100, 1, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a2", 100, "region-2", true, 100, 2, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a3", 100, "region-3", true, 100, 3, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a4", 500, "region-4", false, 0, 0, nil, nil},
	}
	return getTestNodeHostInfo(nhi)
}

func getNodeHostListWithReadyToBeRestoredShard() []pb.NodeHostInfo {
	nhi := []testNodeInfo{
		{"a1", 500, "region-1", true, 100, 1, []uint64{1, 2, 3}, []string{"a1", "a2", "a3"}},
		{"a2", 500, "region-2", false, 0, 0, nil, nil},
		{"a3", 500, "region-3", false, 0, 0, nil, nil},
		{"a4", 500, "region-4", false, 0, 0, nil, nil},
	}
	results := getTestNodeHostInfo(nhi)
	// add LogInfo
	results[1].PlogInfoIncluded = true
	results[1].PlogInfo = append(results[1].PlogInfo, &pb.LogInfo{ShardId: 100, ReplicaId: 2})
	results[2].PlogInfoIncluded = true
	results[2].PlogInfo = append(results[2].PlogInfo, &pb.LogInfo{ShardId: 100, ReplicaId: 3})
	return results
}

//
// other helpers for testing
//

func getRequestedRegion() pb.Regions {
	r := pb.Regions{
		Region: []string{"region-1", "region-2", "region-3"},
		Count:  []uint64{1, 1, 1},
	}
	return r
}

func stringIn(val string, list []string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func uint64In(val uint64, list []uint64) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func TestSchedulerLaunchRequest(t *testing.T) {
	config := GetShardConfig()
	tick := uint64(100)
	nhList := getNodeHostInfoListWithoutAnyShard()
	mnh := getTestMultiNodeHost(nhList)
	mc := getTestMultiShard(nhList)
	shards := getShard()
	regions := getRequestedRegion()
	s := newSchedulerWithContext(nil, config, tick, shards, mc, mnh)
	reqs, err := s.getLaunchRequests(shards, &regions)
	if err != nil {
		t.Errorf("expected to get launch request, error returned: %s", err.Error())
	}
	if len(reqs) != 3 {
		t.Errorf("len(reqs)=%d, want 3", len(reqs))
	}
	for _, req := range reqs {
		if req.Change.Type != pb.Request_CREATE {
			t.Errorf("change type %d, want %d", req.Change.Type, pb.Request_CREATE)
		}
		if req.Change.ShardId != 100 {
			t.Errorf("cluster id %d, want 100", req.Change.ShardId)
		}
		if len(req.Change.Members) != 3 {
			t.Errorf("len(req.Members)=%d, want 3", len(req.Change.Members))
		}
		if len(req.ReplicaIdList) != 3 {
			t.Errorf("len(req.ReplicaIdList)=%d, want 3", len(req.ReplicaIdList))
		}
		if len(req.AddressList) != 3 {
			t.Errorf("len(req.Address)=%d, want 3", len(req.AddressList))
		}
		if !stringIn(req.RaftAddress, []string{"a1", "a2", "a3"}) {
			t.Errorf("unexpected address selected %s", req.RaftAddress)
		}
		if !uint64In(req.InstantiateReplicaId, []uint64{1, 2, 3}) {
			t.Errorf("unexpected node id selected %d", req.InstantiateReplicaId)
		}
		for _, addr := range req.AddressList {
			if !stringIn(addr, []string{"a1", "a2", "a3"}) {
				t.Error("unexpected to be included")
			}
		}
		for _, nid := range req.ReplicaIdList {
			if !uint64In(nid, []uint64{1, 2, 3}) {
				t.Error("unexpected to be included")
			}
		}
	}
}

func TestSchedulerAddRequestDuringRepair(t *testing.T) {
	config := GetShardConfig()
	tick := uint64(500)
	nhList := getNodeHostInfoListWithOneFailedShard()
	mnh := getTestMultiNodeHost(nhList)
	mc := getTestMultiShard(nhList)
	mnh.syncShardInfo(mc)
	shards := getShard()
	s := newSchedulerWithContext(nil, config, tick, shards, mc, mnh)
	if len(s.shardsToRepair) != 1 {
		t.Errorf("len(s.shardsToRepair)=%d, want 1", len(s.shardsToRepair))
	}
	ignoreShards := make(map[uint64]struct{})
	reqs, err := s.repair(ignoreShards)
	if err != nil {
		t.Errorf("error not expected")
	}
	if len(reqs) != 1 {
		t.Errorf("len(reqs)=%d, want 1", len(reqs))
	}
	req := reqs[0]
	if req.Change.Type != pb.Request_ADD {
		t.Errorf("type %d, want %d", req.Change.Type, pb.Request_ADD)
	}
	if req.Change.ShardId != 100 {
		t.Errorf("cluster id %d, want 100", req.Change.ShardId)
	}
	if len(req.Change.Members) != 1 {
		t.Errorf("len(req.Change.Members)=%d, want 1", len(req.Change.Members))
	}
	if uint64In(req.Change.Members[0], []uint64{1, 2, 3}) {
		t.Errorf("re-used node id")
	}
	if req.RaftAddress != "a1" && req.RaftAddress != "a2" {
		t.Errorf("not sending to existing member, address %s", req.RaftAddress)
	}
	if req.AddressList[0] != "a4" {
		t.Errorf("not selecting expected node, %s", req.AddressList[0])
	}
}

func TestSchedulerCreateRequestDuringRepair(t *testing.T) {
	config := GetShardConfig()
	tick := uint64(500)
	nhList := getNodeHostInfoListWithOneAddedShard()
	mnh := getTestMultiNodeHost(nhList)
	mc := getTestMultiShard(nhList)
	shards := getShard()
	s := newSchedulerWithContext(nil, config, tick, shards, mc, mnh)
	if len(s.shardsToRepair) != 1 {
		t.Errorf("len(s.shardsToRepair)=%d, want 1", len(s.shardsToRepair))
	}
	ignoreShards := make(map[uint64]struct{})
	reqs, err := s.repair(ignoreShards)
	if err != nil {
		t.Errorf("error not expected")
	}
	if len(reqs) != 1 {
		t.Errorf("len(reqs)=%d, want 1", len(reqs))
	}
	req := reqs[0]
	if req.Change.Type != pb.Request_CREATE {
		t.Errorf("type %d, want %d", req.Change.Type, pb.Request_CREATE)
	}
	if req.Change.ShardId != 100 {
		t.Errorf("cluster id %d, want 100", req.Change.ShardId)
	}
	if len(req.Change.Members) != 4 {
		t.Errorf("len(req.Change.Members)=%d, want 4", len(req.Change.Members))
	}
	expectedNodeID := []uint64{1, 2, 3, 4}
	if !uint64In(1, expectedNodeID) || !uint64In(2, expectedNodeID) ||
		!uint64In(3, expectedNodeID) || !uint64In(4, expectedNodeID) {
		t.Errorf("unexpected node id list")
	}
	if req.AppName != "noop" {
		t.Errorf("app name %s, want noop", req.AppName)
	}
	if req.InstantiateReplicaId != 4 {
		t.Errorf("req.InstantiateReplicaId=%d, want 4", req.InstantiateReplicaId)
	}
	if req.RaftAddress != "a4" {
		t.Errorf("raft address=%s, want a4", req.RaftAddress)
	}
	if !reflect.DeepEqual(req.ReplicaIdList, req.Change.Members) {
		t.Errorf("expect req.ReplicaIdList == req.Change.Members")
	}
	if len(req.AddressList) != 4 {
		t.Errorf("len(req.AddressList)=%d, want 4", len(req.AddressList))
	}
	if !stringIn("a1", req.AddressList) || !stringIn("a2", req.AddressList) ||
		!stringIn("a3", req.AddressList) || !stringIn("a4", req.AddressList) {
		t.Errorf("unexpected address list element")
	}
}

func TestSchedulerDeleteRequestDuringRepair(t *testing.T) {
	config := GetShardConfig()
	tick := uint64(500)
	nhList := getNodeHostInfoListWithOneReadyForDeleteShard()
	mnh := getTestMultiNodeHost(nhList)
	mc := getTestMultiShard(nhList)
	shards := getShard()
	s := newSchedulerWithContext(nil, config, tick, shards, mc, mnh)
	if len(s.shardsToRepair) != 1 {
		t.Errorf("len(s.shardsToRepair)=%d, want 1", len(s.shardsToRepair))
	}
	ignoreShards := make(map[uint64]struct{})
	reqs, err := s.repair(ignoreShards)
	if err != nil {
		t.Errorf("error not expected")
	}
	if len(reqs) != 1 {
		t.Errorf("len(reqs)=%d, want 1", len(reqs))
	}
	req := reqs[0]
	if req.Change.Type != pb.Request_DELETE {
		t.Errorf("type %d, want %d", req.Change.Type, pb.Request_CREATE)
	}
	if req.Change.ShardId != 100 {
		t.Errorf("cluster id %d, want 100", req.Change.ShardId)
	}
	if len(req.Change.Members) != 1 {
		t.Errorf("len(req.Change.Members)=%d, want 1", len(req.Change.Members))
	}
	if req.Change.Members[0] != 3 {
		t.Errorf("to delete node id %d, want 3", req.Change.Members[0])
	}
	if req.RaftAddress != "a1" && req.RaftAddress != "a2" && req.RaftAddress != "a4" {
		t.Errorf("req.RaftAddress=%s, want a1, a2 or a4", req.RaftAddress)
	}
}

func TestRestoreUnavailableShard(t *testing.T) {
	config := GetShardConfig()
	tick := uint64(500)
	nhList := getNodeHostListWithUnavailableShard()
	mc := getTestMultiShard(nhList)
	newNhList := getNodeHostListWithReadyToBeRestoredShard()
	mnh := getTestMultiNodeHost(newNhList)
	shards := getShard()
	s := newSchedulerWithContext(nil, config, tick, shards, mc, mnh)
	reqs, err := s.restore()
	if err != nil {
		t.Errorf(err.Error())
	}
	if len(reqs) != 2 {
		t.Errorf("reqs sz: %d, want 2", len(reqs))
	}
	for idx := 0; idx < 2; idx++ {
		if reqs[idx].Change.Type != pb.Request_CREATE {
			t.Errorf("change type %d, want %d", reqs[idx].Change.Type, pb.Request_CREATE)
		}
		if reqs[idx].Change.ShardId != 100 {
			t.Errorf("cluster id %d, want 100", reqs[idx].Change.ShardId)
		}
		if len(reqs[idx].Change.Members) != 3 {
			t.Errorf("member sz: %d, want 3", len(reqs[idx].Change.Members))
		}
		if reqs[idx].InstantiateReplicaId != 2 && reqs[idx].InstantiateReplicaId != 3 {
			t.Errorf("new id %d, want 2 or 3", reqs[idx].InstantiateReplicaId)
		}
		if reqs[idx].RaftAddress != "a2" && reqs[idx].RaftAddress != "a3" {
			t.Errorf("new address %s, want a2 or a3", reqs[idx].RaftAddress)
		}
		if len(reqs[idx].ReplicaIdList) != 3 || len(reqs[idx].AddressList) != 3 {
			t.Errorf("cluster info incomplete")
		}
	}
}

func TestRestoreFailedShard(t *testing.T) {
	config := GetShardConfig()
	tick := uint64(500)
	nhList := getNodeHostInfoListWithOneFailedShard()
	mc := getTestMultiShard(nhList)
	newNhList := getNodeHostInfoListWithOneRestartedNode()
	mnh := getTestMultiNodeHost(newNhList)
	shards := getShard()
	s := newSchedulerWithContext(nil, config, tick, shards, mc, mnh)
	reqs, err := s.restore()
	if err != nil {
		t.Errorf(err.Error())
	}
	if len(s.shardsToRepair) != 1 {
		t.Fatalf("suppose to have one cluster need to be repaired")
	}
	if s.shardsToRepair[0].needToBeRestored() {
		t.Errorf("not suppose to be marked as needToBeRestored")
	}
	if len(reqs) != 1 {
		t.Errorf("got %d reqs, want 1", len(reqs))
	}
	if reqs[0].Change.Type != pb.Request_CREATE {
		t.Errorf("expected to have a CREATE request")
	}
	if reqs[0].Change.ShardId != 100 {
		t.Errorf("cluster id %d, want 100", reqs[0].Change.ShardId)
	}
	if len(reqs[0].Change.Members) != 3 {
		t.Errorf("member sz: %d, want 3", len(reqs[0].Change.Members))
	}
	if reqs[0].InstantiateReplicaId != 3 {
		t.Errorf("new id %d, want 3", reqs[0].InstantiateReplicaId)
	}
	if reqs[0].RaftAddress != "a3" {
		t.Errorf("new address %s, want a3", reqs[0].RaftAddress)
	}
	if len(reqs[0].ReplicaIdList) != 3 || len(reqs[0].AddressList) != 3 {
		t.Errorf("cluster info incomplete")
	}
}

func TestKillZombieReplicas(t *testing.T) {
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
	s := &scheduler{
		replicasToKill: mc.getToKillReplicas(),
	}
	reqs := s.killZombieReplicas()
	if len(reqs) != 1 {
		t.Fatalf("failed to generate kill zombie request")
	}
	req := reqs[0]
	if req.Change.Type != pb.Request_KILL {
		t.Errorf("unexpected type %s", req.Change.Type)
	}
	if req.Change.ShardId != 1 {
		t.Errorf("cluster id %d, want 1", req.Change.ShardId)
	}
	if req.Change.Members[0] != 4 {
		t.Errorf("node id %d, want 4", req.Change.Members[0])
	}
	if req.RaftAddress != "a4" {
		t.Errorf("address %s, want a4", req.RaftAddress)
	}
}

func TestLaunchRequestsForLargeNumberOfShardsCanBeDelivered(t *testing.T) {
	numOfShards := 10000
	cl := make([]*pb.Shard, 0)
	for i := 0; i < numOfShards; i++ {
		c := &pb.Shard{
			AppName: random.String(32),
			ShardId: rand.Uint64(),
			Members: []uint64{
				rand.Uint64(),
				rand.Uint64(),
				rand.Uint64(),
				rand.Uint64(),
				rand.Uint64(),
			},
		}
		cl = append(cl, c)
	}
	regions := pb.Regions{
		Region: []string{
			random.String(64),
			random.String(64),
			random.String(64),
			random.String(64),
			random.String(64),
		},
		Count: []uint64{1, 1, 1, 1, 1},
	}
	nhList := make([]*nodeHostSpec, 0)
	for i := 0; i < 5; i++ {
		nh := &nodeHostSpec{
			Address:    random.String(260),
			RPCAddress: random.String(260),
			Region:     regions.Region[i],
			Shards:     make(map[uint64]struct{}),
		}
		nhList = append(nhList, nh)
	}
	s := scheduler{}
	s.randomSrc = random.NewLockedRand()
	s.nodeHostList = nhList
	s.config = getDefaultShardConfig()
	reqs, err := s.getLaunchRequests(cl, &regions)
	if err != nil {
		t.Errorf("failed to get launch requests, %v", err)
	}
	for i := 0; i < 5; i++ {
		addr := nhList[i].Address
		selected := make([]*pb.NodeHostRequest, 0)
		for _, req := range reqs {
			v := *req
			if v.RaftAddress == addr {
				selected = append(selected, &v)
			}
		}
		rc := pb.NodeHostRequestCollection{
			Requests: selected,
		}
		data, err := proto.Marshal(&rc)
		if err != nil {
			t.Fatalf("failed to marshal")
		}
		sz := uint64(len(data))
		plog.Infof("for nodehost %d, size : %d", i, sz)
		if sz > settings.Soft.MaxDrummerServerMsgSize ||
			sz > settings.Soft.MaxDrummerClientMsgSize {
			t.Errorf("message is too big to be delivered")
		}
	}
}
