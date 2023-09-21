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

package drummer

import (
	pb "github.com/lni/drummer/v3/drummerpb"
	"github.com/lni/goutils/logutil"
)

type nodeHostSpec struct {
	Address       string
	RPCAddress    string
	Region        string
	Tick          uint64
	PersistentLog []*pb.LogInfo
	Shards        map[uint64]struct{}
}

func (spec *nodeHostSpec) deepCopy() *nodeHostSpec {
	ns := &nodeHostSpec{
		Address: spec.Address,
		Region:  spec.Region,
		Tick:    spec.Tick,
	}
	ns.PersistentLog = make([]*pb.LogInfo, 0)
	ns.PersistentLog = append(ns.PersistentLog, spec.PersistentLog...)
	ns.Shards = make(map[uint64]struct{})
	for k, v := range spec.Shards {
		ns.Shards[k] = v
	}
	return ns
}

func (spec *nodeHostSpec) hasLog(shardID uint64, replicaID uint64) bool {
	for _, v := range spec.PersistentLog {
		if v.ShardId == shardID && v.ReplicaId == replicaID {
			return true
		}
	}
	return false
}

func (spec *nodeHostSpec) available(currentTick uint64) bool {
	return !EntityFailed(spec.Tick, currentTick)
}

type multiNodeHost struct {
	Nodehosts map[string]*nodeHostSpec
}

func newMultiNodeHost() *multiNodeHost {
	return &multiNodeHost{
		Nodehosts: make(map[string]*nodeHostSpec),
	}
}

func (m *multiNodeHost) get(address string) *nodeHostSpec {
	return m.Nodehosts[address]
}

func (m *multiNodeHost) update(nhi pb.NodeHostInfo) {
	if _, ok := m.Nodehosts[nhi.RaftAddress]; ok {
		m.syncNodeHostSpec(nhi)
	} else {
		m.Nodehosts[nhi.RaftAddress] = m.toNodeHostSpec(nhi)
	}
}

func (m *multiNodeHost) syncShardInfo(mc *multiShard) {
	for cid, shard := range mc.Shards {
		for _, node := range shard.Replicas {
			if spec, ok := m.Nodehosts[node.Address]; ok {
				spec.Shards[cid] = struct{}{}
			}
		}
	}
}

func (m *multiNodeHost) deepCopy() *multiNodeHost {
	nm := &multiNodeHost{}
	nm.Nodehosts = make(map[string]*nodeHostSpec)
	for k, v := range m.Nodehosts {
		nm.Nodehosts[k] = v.deepCopy()
	}
	return nm
}

func (m *multiNodeHost) toArray() []*nodeHostSpec {
	result := make([]*nodeHostSpec, 0)
	for _, v := range m.Nodehosts {
		result = append(result, v)
	}
	return result
}

func (m *multiNodeHost) toNodeHostSpec(nhi pb.NodeHostInfo) *nodeHostSpec {
	n := &nodeHostSpec{
		Address:    nhi.RaftAddress,
		RPCAddress: nhi.RPCAddress,
		Region:     nhi.Region,
		Tick:       nhi.LastTick,
	}
	n.PersistentLog = make([]*pb.LogInfo, 0)
	n.Shards = make(map[uint64]struct{})
	if nhi.PlogInfoIncluded {
		n.PersistentLog = append(n.PersistentLog, nhi.PlogInfo...)
	}
	for _, cid := range nhi.ShardIdList {
		n.Shards[cid] = struct{}{}
	}
	return n
}

func (m *multiNodeHost) syncNodeHostSpec(nhi pb.NodeHostInfo) {
	spec, ok := m.Nodehosts[nhi.RaftAddress]
	if !ok {
		panic("nodeHostSpec not found")
	}
	spec.Region = nhi.Region
	spec.Tick = nhi.LastTick
	if nhi.PlogInfoIncluded {
		if len(nhi.PlogInfo) == 0 && len(spec.PersistentLog) > 0 {
			for _, plv := range spec.PersistentLog {
				plog.Debugf("PersisentLog %s is lost on %s, replaced disk?",
					logutil.DescribeNode(plv.ShardId, plv.ReplicaId), nhi.RaftAddress)
			}
		}
		spec.PersistentLog = make([]*pb.LogInfo, 0)
		spec.PersistentLog = append(spec.PersistentLog, nhi.PlogInfo...)
	}
	cm := make(map[uint64]struct{})
	for _, cid := range nhi.ShardIdList {
		cm[cid] = struct{}{}
	}
	spec.Shards = cm
}
