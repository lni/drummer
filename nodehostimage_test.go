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

func TestMultiNodeHostDeepCopy(t *testing.T) {
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 1,
	}
	pl := &pb.LogInfo{
		ShardId:   100,
		ReplicaId: 200,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress: "a2",
		LastTick:    100,
		Region:      "region-1",
		ShardInfo:   []*pb.ShardInfo{ci},
		PlogInfo:    []*pb.LogInfo{pl},
	}
	mnh := newMultiNodeHost()
	mnh.update(nhi)
	dcmnh := mnh.deepCopy()
	if !reflect.DeepEqual(dcmnh, mnh) {
		t.Errorf("deep equal failed")
	}
	dcmnh.Nodehosts["a2"].Tick = 200
	if dcmnh.Nodehosts["a2"].Tick == mnh.Nodehosts["a2"].Tick {
		t.Errorf("deep copy is not actually deep copy")
	}
}

func TestMultiNodeHostCanForgetOldLogInfo(t *testing.T) {
	ci := &pb.ShardInfo{
		ShardId:           1,
		ReplicaId:         2,
		IsLeader:          true,
		Replicas:          map[uint64]string{1: "a1", 2: "a2", 3: "a3"},
		ConfigChangeIndex: 1,
	}
	pl := &pb.LogInfo{
		ShardId:   100,
		ReplicaId: 200,
	}
	nhi := pb.NodeHostInfo{
		RaftAddress:      "a2",
		LastTick:         100,
		Region:           "region-1",
		PlogInfoIncluded: true,
		ShardInfo:        []*pb.ShardInfo{ci},
		PlogInfo:         []*pb.LogInfo{pl},
	}
	mnh := newMultiNodeHost()
	mnh.update(nhi)
	if !mnh.Nodehosts["a2"].hasLog(pl.ShardId, pl.ReplicaId) {
		t.Errorf("plog info not found")
	}
	// when PlogInfoIncluded flag is false
	// update() won't update the persistentLog recorded
	nhi.PlogInfoIncluded = false
	nhi.PlogInfo = []*pb.LogInfo{}
	mnh.update(nhi)
	if !mnh.Nodehosts["a2"].hasLog(pl.ShardId, pl.ReplicaId) {
		t.Errorf("plog info not found")
	}
	pl2 := &pb.LogInfo{
		ShardId:   200,
		ReplicaId: 500,
	}
	nhi.PlogInfoIncluded = true
	nhi.PlogInfo = []*pb.LogInfo{pl2}
	mnh.update(nhi)
	if mnh.Nodehosts["a2"].hasLog(pl.ShardId, pl.ReplicaId) {
		t.Errorf("plog info unexpected")
	}
	if !mnh.Nodehosts["a2"].hasLog(pl2.ShardId, pl2.ReplicaId) {
		t.Errorf("plog info not found")
	}
}
