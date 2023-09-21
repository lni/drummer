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
	"bytes"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/lni/dragonboat/v4/statemachine"
	pb "github.com/lni/drummer/v3/drummerpb"
)

func TestDBCanBeSnapshottedAndRestored(t *testing.T) {
	ci1 := pb.Shard{
		ShardId: 100,
		AppName: "noop",
		Members: []uint64{1, 2, 3},
	}
	ci2 := pb.Shard{
		ShardId: 200,
		AppName: "noop",
		Members: []uint64{1, 2, 3},
	}
	shards := make(map[uint64]*pb.Shard)
	shards[100] = &ci1
	shards[200] = &ci2
	testData := make([]byte, 32)
	rand.Read(testData)
	kvMap := make(map[string][]byte)
	kvMap["key1"] = []byte("value1")
	kvMap["key2"] = []byte("value2")
	kvMap["key3"] = testData
	d := &DB{
		ShardID:       0,
		ReplicaID:     0,
		Shards:        shards,
		KVMap:         kvMap,
		ShardImage:    newMultiShard(),
		NodeHostImage: newMultiNodeHost(),
		NodeHostInfo:  make(map[string]pb.NodeHostInfo),
		Requests:      make(map[string][]*pb.NodeHostRequest),
		Outgoing:      make(map[string][]*pb.NodeHostRequest),
	}
	testRequestsCanBeUpdated(t, d)
	testNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t, d)
	w := bytes.NewBufferString("")
	err := d.SaveSnapshot(w, nil, nil)
	if err != nil {
		t.Fatalf("failed to get snapshot")
	}
	hash1, _ := d.GetHash()
	r := bytes.NewBuffer(w.Bytes())
	newDB := &DB{}
	err = newDB.RecoverFromSnapshot(r, nil, nil)
	if err != nil {
		t.Fatalf("failed to recover from snapshot")
	}
	if !reflect.DeepEqual(newDB, d) {
		t.Errorf("recovered drummerdb not equal to the original, %v \n\n\n %v",
			newDB, d)
	}
	hash2, _ := newDB.GetHash()
	if hash1 != hash2 {
		t.Errorf("hash value mismatch")
	}
}

func testTickCanBeIncreased(t *testing.T, db statemachine.IStateMachine) {
	update := pb.Update{
		Type: pb.Update_TICK,
	}
	data, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}

	entry := statemachine.Entry{
		Cmd: data,
	}

	v1, err := db.Update(entry)
	if err != nil {
		t.Fatalf("%v", err)
	}
	v2, err := db.Update(entry)
	if err != nil {
		t.Fatalf("%v", err)
	}
	v3, err := db.Update(entry)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if v2.Value != v1.Value+tickIntervalSecond ||
		v3.Value != v2.Value+tickIntervalSecond ||
		v3.Value != 3*tickIntervalSecond {
		t.Errorf("not returning expected tick value")
	}
	if db.(*DB).Tick != 3*tickIntervalSecond {
		t.Errorf("unexpected tick value")
	}
	v4, _ := db.Update(statemachine.Entry{Cmd: data})
	if v4.Value != 4*tickIntervalSecond || db.(*DB).Tick != 4*tickIntervalSecond {
		t.Errorf("unexpected tick value")
	}
}

func TestTickCanBeIncreased(t *testing.T) {
	db := NewDB(0, 0)
	testTickCanBeIncreased(t, db)
}

func testNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t *testing.T,
	db statemachine.IStateMachine) {
	update := pb.Update{
		Type: pb.Update_TICK,
	}
	tickData, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}
	nhi := &pb.NodeHostInfo{
		RaftAddress: "a2",
		RPCAddress:  "rpca1",
		Region:      "r1",
		ShardInfo: []*pb.ShardInfo{
			&pb.ShardInfo{
				ShardId:   1,
				ReplicaId: 2,
			},
			&pb.ShardInfo{
				ShardId:   2,
				ReplicaId: 3,
				Replicas:  map[uint64]string{3: "a2", 4: "a1", 5: "a3"},
			},
		},
	}
	update = pb.Update{
		Type:         pb.Update_NODEHOST_INFO,
		NodehostInfo: nhi,
	}
	data, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}
	if _, err := db.Update(statemachine.Entry{Cmd: tickData}); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := db.Update(statemachine.Entry{Cmd: tickData}); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := db.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	shardInfo := db.(*DB).ShardImage
	nodehostInfo := db.(*DB).NodeHostImage
	if len(shardInfo.Shards) != 2 {
		t.Errorf("shard info not updated")
	}
	if len(nodehostInfo.Nodehosts) != 1 {
		t.Errorf("nodehost info not updated")
	}
	nhs, ok := nodehostInfo.Nodehosts["a2"]
	if !ok {
		t.Errorf("nodehost not found")
	}
	if nhs.Tick != 2*tickIntervalSecond {
		t.Errorf("unexpected nodehost tick value")
	}
	ci, ok := shardInfo.Shards[2]
	if !ok {
		t.Errorf("shard not found")
	}
	if ci.ShardID != 2 {
		t.Errorf("unexpected shard id value")
	}
	n, ok := ci.Replicas[3]
	if !ok {
		t.Errorf("node not found, %v", ci)
	}
	if n.Tick != 2*tickIntervalSecond {
		t.Errorf("unexpected node tick value")
	}
}

func TestNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t *testing.T) {
	db := NewDB(0, 0)
	testNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t, db)
}

func testTickValue(t *testing.T, db statemachine.IStateMachine,
	addr string, shardID uint64, nodeID uint64, tick uint64) {
	shardInfo := db.(*DB).ShardImage
	nodehostInfo := db.(*DB).NodeHostImage
	nhs, ok := nodehostInfo.Nodehosts[addr]
	if !ok {
		t.Fatalf("nodehost %s not found", addr)
	}
	if nhs.Tick != tick*tickIntervalSecond {
		t.Errorf("unexpected nodehost tick value, got %d, want %d", nhs.Tick, tick)
	}
	ci, ok := shardInfo.Shards[shardID]
	if !ok {
		t.Fatalf("shard %d not found", shardID)
	}
	n, ok := ci.Replicas[nodeID]
	if !ok {
		t.Fatalf("node %d not found", nodeID)
	}
	if n.Tick != tick*tickIntervalSecond {
		t.Errorf("unexpected node tick value, got %d, want %d", n.Tick, tick)
	}
}

func TestTickUpdatedAsExpected(t *testing.T) {
	db := NewDB(0, 0)
	update := pb.Update{
		Type: pb.Update_TICK,
	}
	tickData, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		RPCAddress:  "rpca1",
		Region:      "r1",
		ShardInfo: []*pb.ShardInfo{
			&pb.ShardInfo{
				ShardId:   1,
				ReplicaId: 2,
				Replicas:  map[uint64]string{2: "a2", 3: "a1", 4: "a3"},
			},
		},
	}
	update = pb.Update{
		Type:         pb.Update_NODEHOST_INFO,
		NodehostInfo: &nhi,
	}
	nhiData, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}
	for i := uint64(0); i < 10; i++ {
		if _, err := db.Update(statemachine.Entry{Cmd: tickData}); err != nil {
			t.Fatalf("%v", err)
		}
		if _, err := db.Update(statemachine.Entry{Cmd: nhiData}); err != nil {
			t.Fatalf("%v", err)
		}
		testTickValue(t, db, "a2", 1, 2, i+1)
	}
	for i := uint64(0); i < 10; i++ {
		if _, err := db.Update(statemachine.Entry{Cmd: nhiData}); err != nil {
			t.Fatalf("%v", err)
		}
		testTickValue(t, db, "a2", 1, 2, 10)
	}
	for i := uint64(0); i < 10; i++ {
		if _, err := db.Update(statemachine.Entry{Cmd: tickData}); err != nil {
			t.Fatalf("%v", err)
		}
		if _, err := db.Update(statemachine.Entry{Cmd: nhiData}); err != nil {
			t.Fatalf("%v", err)
		}
		testTickValue(t, db, "a2", 1, 2, 11+i)
	}
}

func TestNodeHostInfoUpdateMovesRequests(t *testing.T) {
	db := NewDB(0, 0)
	testRequestsCanBeUpdated(t, db)
	testNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t, db)
	reqs := db.(*DB).Requests
	if len(reqs) != 1 {
		t.Errorf("unexpected reqs size")
	}
	outgoings := db.(*DB).Outgoing
	if len(outgoings) != 1 {
		t.Errorf("unexpected outgoing size")
	}
	r, ok := outgoings["a2"]
	if !ok {
		t.Errorf("requests not in the outgoing map")
	}
	if len(r) != 2 {
		t.Errorf("requests size mismatch")
	}
	_, ok = reqs["a2"]
	if ok {
		t.Errorf("not expected to have reqs for a2 in requests")
	}
	q, ok := reqs["a1"]
	if !ok {
		t.Errorf("requests missing for a1")
	}
	if len(q) != 1 {
		t.Errorf("unexpected size")
	}
}

func testRequestsCanBeUpdated(t *testing.T, db statemachine.IStateMachine) {
	r1 := &pb.NodeHostRequest{
		Change:               &pb.Request{},
		RaftAddress:          "a1",
		InstantiateReplicaId: 1,
	}
	r2 := &pb.NodeHostRequest{
		Change:               &pb.Request{},
		RaftAddress:          "a2",
		InstantiateReplicaId: 2,
	}
	r3 := &pb.NodeHostRequest{
		Change:               &pb.Request{},
		RaftAddress:          "a2",
		InstantiateReplicaId: 3,
	}
	update := pb.Update{
		Type: pb.Update_REQUESTS,
		Requests: &pb.NodeHostRequestCollection{
			Requests: []*pb.NodeHostRequest{r1, r2, r3},
		},
	}
	data, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}
	v, _ := db.Update(statemachine.Entry{Cmd: data})
	if v.Value != 3 {
		t.Errorf("unexpected returned value")
	}
	reqs := db.(*DB).Requests
	if len(reqs) != 2 {
		t.Errorf("reqs size unexpected")
	}
	q1, ok := reqs["a1"]
	if !ok {
		t.Errorf("requests for a1 not found")
	}
	if len(q1) != 1 {
		t.Errorf("unexpected request size for a1")
	}
	q2, ok := reqs["a2"]
	if !ok {
		t.Errorf("requests for a2 not found")
	}
	if len(q2) != 2 {
		t.Errorf("unexpected request size for a2")
	}
}

func TestReqeustsCanBeUpdated(t *testing.T) {
	db := NewDB(0, 0)
	testRequestsCanBeUpdated(t, db)
}

func TestRequestLookup(t *testing.T) {
	db := NewDB(0, 0)
	testRequestsCanBeUpdated(t, db)
	lookup := pb.LookupRequest{
		Type:    pb.LookupRequest_REQUESTS,
		Address: "a2",
	}
	data, err := proto.Marshal(&lookup)
	if err != nil {
		panic(err)
	}
	result, _ := db.Lookup(data)
	var resp pb.LookupResponse
	if err := proto.Unmarshal(result.([]byte), &resp); err != nil {
		panic(err)
	}
	if len(resp.Requests.Requests) != 0 {
		t.Errorf("unexpected requests count, want 0, got %d",
			len(resp.Requests.Requests))
	}
	testNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t, db)
	result, _ = db.Lookup(data)
	if err := proto.Unmarshal(result.([]byte), &resp); err != nil {
		panic(err)
	}
	reqs := resp.Requests.Requests
	if len(reqs) != 2 {
		t.Errorf("unexpected requests count, want 2, got %d",
			len(resp.Requests.Requests))
	}
	if reqs[0].InstantiateReplicaId != 2 || reqs[1].InstantiateReplicaId != 3 {
		t.Errorf("requests not ordered in the expected way")
	}
}

func TestSchedulerContextLookup(t *testing.T) {
	db := NewDB(0, 0)
	regions := pb.Regions{
		Region: []string{"v1", "v2"},
		Count:  []uint64{123, 345},
	}
	regionData, err := proto.Marshal(&regions)
	if err != nil {
		panic(err)
	}
	var kv pb.KV
	kv.Value = string(regionData)
	rd, err := proto.Marshal(&kv)
	if err != nil {
		panic(err)
	}
	db.(*DB).KVMap[regionsKey] = rd
	db.(*DB).Shards[100] = &pb.Shard{
		ShardId: 100,
	}
	db.(*DB).Shards[200] = &pb.Shard{
		ShardId: 200,
	}
	db.(*DB).Shards[300] = &pb.Shard{
		ShardId: 300,
	}
	testRequestsCanBeUpdated(t, db)
	testNodeHostInfoUpdateUpdatesShardAndNodeHostImage(t, db)
	lookup := pb.LookupRequest{
		Type: pb.LookupRequest_SCHEDULER_CONTEXT,
	}
	data, err := proto.Marshal(&lookup)
	if err != nil {
		panic(err)
	}
	result, _ := db.Lookup(data)
	var sc schedulerContext
	if err := json.Unmarshal(result.([]byte), &sc); err != nil {
		panic(err)
	}
	if sc.Tick != 2*tickIntervalSecond {
		t.Errorf("tick %d, want 2", sc.Tick)
	}
	shardInfo := sc.ShardImage
	nodehostInfo := sc.NodeHostImage
	if len(sc.ShardImage.Shards) != 2 {
		t.Fatalf("shard info not updated, sz: %d", len(shardInfo.Shards))
	}
	if len(nodehostInfo.Nodehosts) != 1 {
		t.Fatalf("nodehost info not updated")
	}
	nhs, ok := nodehostInfo.Nodehosts["a2"]
	if !ok {
		t.Errorf("nodehost not found")
	}
	if nhs.Tick != 2*tickIntervalSecond {
		t.Errorf("unexpected nodehost tick value")
	}
	ci, ok := shardInfo.Shards[2]
	if !ok {
		t.Fatalf("shard not found")
	}
	if ci.ShardID != 2 {
		t.Errorf("unexpected shard id value")
	}
	_, ok = ci.Replicas[3]
	if !ok {
		t.Fatalf("node not found, %v", ci)
	}
	if len(sc.Regions.Region) != 2 || len(sc.Regions.Count) != 2 {
		t.Errorf("regions not returned")
	}
	if len(sc.Shards) != 3 {
		t.Errorf("shards not returned")
	}
}

func TestShardCanBeUpdatedAndLookedUp(t *testing.T) {
	change := &pb.Change{
		Type:    pb.Change_CREATE,
		ShardId: 123,
		Members: []uint64{1, 2, 3},
		AppName: "noop",
	}
	du := pb.Update{
		Type:   pb.Update_SHARD,
		Change: change,
	}
	data, err := proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	oldData := data
	d := NewDB(0, 0)
	code, _ := d.Update(statemachine.Entry{Cmd: data})
	if code.Value != DBUpdated {
		t.Errorf("code %d, want %d", code, DBUpdated)
	}
	// use the same input to update the drummer db again
	code, _ = d.Update(statemachine.Entry{Cmd: data})
	if code.Value != ShardExists {
		t.Errorf("code %d, want %d", code, ShardExists)
	}
	// set the bootstrapped flag
	bkv := &pb.KV{
		Key:       bootstrappedKey,
		Value:     "bootstrapped",
		Finalized: true,
	}
	du = pb.Update{
		Type:     pb.Update_KV,
		KvUpdate: bkv,
	}
	data, err = proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	if !d.(*DB).bootstrapped() {
		t.Errorf("not bootstrapped")
	}
	code, _ = d.Update(statemachine.Entry{Cmd: oldData})
	if code.Value != DBBootstrapped {
		t.Errorf("code %d, want %d", code, DBBootstrapped)
	}
	if len(d.(*DB).Shards) != 1 {
		t.Fatalf("failed to create shard")
	}
	c := d.(*DB).Shards[123]
	shard := &pb.Shard{
		ShardId: 123,
		AppName: "noop",
		Members: []uint64{1, 2, 3},
	}
	if !reflect.DeepEqual(shard, c) {
		t.Errorf("shard recs not equal, \n%v\n%v", shard, c)
	}
	req := pb.LookupRequest{
		Type: pb.LookupRequest_SHARD,
	}
	data, err = proto.Marshal(&req)
	if err != nil {
		t.Fatalf("failed to marshal lookup request")
	}
	result, err := d.Lookup(data)
	if err != nil {
		t.Fatalf("lookup failed %v", err)
	}
	var resp pb.LookupResponse
	err = proto.Unmarshal(result.([]byte), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal")
	}
	if len(resp.Shards) != 1 {
		t.Fatalf("not getting shard back")
	}
	resqc := resp.Shards[0]
	if resqc.ShardId != 123 {
		t.Errorf("shard id %d, want 123", resqc.ShardId)
	}
	if len(resqc.Members) != 3 {
		t.Errorf("len(members)=%d, want 3", len(resqc.Members))
	}
	if resqc.AppName != "noop" {
		t.Errorf("app name %s, want noop", resqc.AppName)
	}
}

func TestKVMapCanBeUpdatedAndLookedUpForFinalizedValue(t *testing.T) {
	kv := &pb.KV{
		Key:       "test-key",
		Value:     "test-data",
		Finalized: true,
	}
	du := pb.Update{
		Type:     pb.Update_KV,
		KvUpdate: kv,
	}
	data, err := proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	d := NewDB(0, 0)
	code, err := d.Update(statemachine.Entry{Cmd: data})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if code.Value != DBKVUpdated {
		t.Errorf("code %d, want %d", code, DBKVUpdated)
	}
	// apply the same update again, suppose to be rejected
	code, err = d.Update(statemachine.Entry{Cmd: data})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if code.Value != DBKVFinalized {
		t.Errorf("code %d, want %d", code, DBKVFinalized)
	}
	if len(d.(*DB).KVMap) != 1 {
		t.Errorf("kv map not updated")
	}
	v, ok := d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// v is the marshaled how KV rec
	var kvrec pb.KV
	if err := proto.Unmarshal([]byte(v), &kvrec); err != nil {
		panic(err)
	}
	if kvrec.Value != "test-data" {
		t.Errorf("value %s, want test-data", kvrec.Value)
	}
	// try to update the finalized value
	du.KvUpdate.Value = "test-data-2"
	data, err = proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	v, ok = d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// finalized value not changed
	if err := proto.Unmarshal([]byte(v), &kvrec); err != nil {
		panic(err)
	}
	if kvrec.Value != "test-data" {
		t.Errorf("value %s, want test-data", kvrec.Value)
	}
}

func TestKVMapCanBeUpdatedAndLookedUpForNotFinalizedValue(t *testing.T) {
	kv := &pb.KV{
		Key:        "test-key",
		Value:      "test-data",
		InstanceId: 1000,
	}
	du := pb.Update{
		Type:     pb.Update_KV,
		KvUpdate: kv,
	}
	data, err := proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	d := NewDB(0, 0)
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	if len(d.(*DB).KVMap) != 1 {
		t.Errorf("kv map not updated")
	}
	_, ok := d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// with a different instance id, the value can not be updated
	du.KvUpdate.Value = "test-data-2"
	du.KvUpdate.InstanceId = 2000
	data, err = proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	v, ok := d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// finalized value not changed
	var kvrec pb.KV
	if err := proto.Unmarshal([]byte(v), &kvrec); err != nil {
		panic(err)
	}
	if kvrec.Value != "test-data" {
		t.Errorf("value %s, want test-data", kvrec.Value)
	}
	// with the same instance id, the value should be updated
	du.KvUpdate.InstanceId = 1000
	data, err = proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	v, ok = d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// finalized value not changed
	if err := proto.Unmarshal([]byte(v), &kvrec); err != nil {
		panic(err)
	}
	if kvrec.Value != "test-data-2" {
		t.Errorf("value %s, want test-data", kvrec.Value)
	}
	// with old instance id value not matching the instance id, value
	// should not be updated
	du.KvUpdate.OldInstanceId = 3000
	du.KvUpdate.InstanceId = 0
	du.KvUpdate.Value = "test-data-3"
	data, err = proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	v, ok = d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// finalized value not changed
	if err := proto.Unmarshal([]byte(v), &kvrec); err != nil {
		panic(err)
	}
	if kvrec.Value != "test-data-2" {
		t.Errorf("value %s, want test-data", kvrec.Value)
	}
	// with old instance id value matching the instance id, value should
	// be updated
	du.KvUpdate.OldInstanceId = 1000
	du.KvUpdate.InstanceId = 0
	du.KvUpdate.Value = "test-data-3"
	data, err = proto.Marshal(&du)
	if err != nil {
		t.Fatalf("failed to marshal")
	}
	if _, err := d.Update(statemachine.Entry{Cmd: data}); err != nil {
		t.Fatalf("%v", err)
	}
	v, ok = d.(*DB).KVMap["test-key"]
	if !ok {
		t.Errorf("kv map value not expected")
	}
	// finalized value not changed
	if err := proto.Unmarshal([]byte(v), &kvrec); err != nil {
		panic(err)
	}
	if kvrec.Value != "test-data-3" {
		t.Errorf("value %s, want test-data", kvrec.Value)
	}
}

func expectPanic(t *testing.T) {
	if r := recover(); r == nil {
		t.Fatalf("expected to panic, but it didn't panic")
	}
}

func TestDBCanBeMarkedAsFailed(t *testing.T) {
	db := NewDB(0, 1)
	db.(*DB).Failed = true
	defer expectPanic(t)
	if _, err := db.Update(statemachine.Entry{}); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestLaunchDeadlineIsChecked(t *testing.T) {
	db := NewDB(0, 1)
	db.(*DB).LaunchDeadline = 100
	defer expectPanic(t)
	for i := 0; i <= 100; i++ {
		db.(*DB).applyTickUpdate()
	}
}

func TestLaunchRequestSetsTheLaunchFlag(t *testing.T) {
	db := NewDB(0, 1)
	r1 := &pb.NodeHostRequest{
		RaftAddress:          "a1",
		InstantiateReplicaId: 1,
		Join:                 false,
		Restore:              false,
		Change: &pb.Request{
			Type: pb.Request_CREATE,
		},
	}
	update := pb.Update{
		Type: pb.Update_REQUESTS,
		Requests: &pb.NodeHostRequestCollection{
			Requests: []*pb.NodeHostRequest{r1},
		},
	}
	data, err := proto.Marshal(&update)
	if err != nil {
		panic(err)
	}
	for i := 0; i <= 100; i++ {
		db.(*DB).applyTickUpdate()
	}
	if db.(*DB).launched() {
		t.Fatalf("launched flag already unexpectedly set")
	}
	v, _ := db.Update(statemachine.Entry{Cmd: data})
	if v.Value != 1 {
		t.Errorf("unexpected returned value")
	}
	if !db.(*DB).launched() {
		t.Fatalf("launched flag is not set")
	}
	if db.(*DB).LaunchDeadline != (101+launchDeadlineTick)*tickIntervalSecond {
		t.Errorf("deadline not set, deadline %d, tick %d",
			db.(*DB).LaunchDeadline, launchDeadlineTick)
	}
	if db.(*DB).Failed {
		t.Errorf("failed flag already set")
	}
}

func TestLaunchDeadlineIsClearedOnceAllNodesAreLaunched(t *testing.T) {
	db := NewDB(0, 1)
	db.(*DB).Shards[1] = nil
	db.(*DB).LaunchDeadline = 100
	ci1 := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         1,
		Replicas:          map[uint64]string{1: "a1", 2: "a2"},
		ConfigChangeIndex: 3,
	}
	u1 := &pb.Update{
		NodehostInfo: &pb.NodeHostInfo{
			RaftAddress: "a1",
			ShardInfo:   []*pb.ShardInfo{ci1},
		},
		Type: pb.Update_NODEHOST_INFO,
	}
	data1, err := proto.Marshal(u1)
	if err != nil {
		panic(err)
	}
	ci2 := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		Replicas:          map[uint64]string{1: "a1", 2: "a2"},
		ConfigChangeIndex: 3,
	}
	u2 := pb.Update{
		NodehostInfo: &pb.NodeHostInfo{
			RaftAddress: "a2",
			ShardInfo:   []*pb.ShardInfo{ci2},
		},
		Type: pb.Update_NODEHOST_INFO,
	}
	data2, err := proto.Marshal(&u2)
	if err != nil {
		panic(err)
	}
	db.(*DB).applyTickUpdate()
	if _, err := db.Update(statemachine.Entry{Cmd: data1}); err != nil {
		t.Fatalf("%v", err)
	}
	m := db.(*DB).getLaunchedShards()
	if len(m) != 0 {
		t.Fatalf("launched shard is not 0")
	}
	if db.(*DB).LaunchDeadline != 100 {
		t.Errorf("deadline is not cleared")
	}
	if _, err := db.Update(statemachine.Entry{Cmd: data2}); err != nil {
		t.Fatalf("%v", err)
	}
	m = db.(*DB).getLaunchedShards()
	if len(m) != 1 {
		t.Fatalf("launched shard is not 1")
	}
	if db.(*DB).LaunchDeadline != 0 {
		t.Errorf("deadline is not cleared")
	}
}
