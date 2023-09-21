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
	"sort"

	pb "github.com/lni/drummer/v3/drummerpb"
	"github.com/lni/drummer/v3/settings"
	"github.com/lni/goutils/logutil"
)

var (
	// nodeHostTTL defines the number of seconds without heartbeat required to
	// consider a node host as dead.
	nodeHostTTL = settings.Soft.NodeHostTTL
)

type replica struct {
	ShardID       uint64
	ReplicaID     uint64
	Address       string
	IsLeader      bool
	Tick          uint64
	FirstObserved uint64
}

type shard struct {
	ShardID           uint64
	ConfigChangeIndex uint64
	Replicas          map[uint64]*replica
}

type multiShard struct {
	Shards         map[uint64]*shard
	ReplicasToKill []replicaToKill
}

type shardRepair struct {
	shardID         uint64
	shard           *shard
	failedReplicas  []replica
	okReplicas      []replica
	replicasToStart []replica
}

type replicaToKill struct {
	ShardID   uint64
	ReplicaID uint64
	Address   string
}

func (cr *shardRepair) id() string {
	return logutil.ShardID(cr.shardID)
}

func (cr *shardRepair) quorum() int {
	count := len(cr.failedReplicas) + len(cr.okReplicas) + len(cr.replicasToStart)
	return count/2 + 1
}

func (cr *shardRepair) available() bool {
	return len(cr.okReplicas) >= cr.quorum()
}

func (cr *shardRepair) addRequired() bool {
	return len(cr.failedReplicas) > 0 &&
		len(cr.replicasToStart) == 0 && cr.available()
}

func (cr *shardRepair) createRequired() bool {
	return len(cr.replicasToStart) > 0
}

func (cr *shardRepair) deleteRequired(expectedShardSize int) bool {
	return cr.available() && len(cr.failedReplicas) > 0 &&
		len(cr.failedReplicas)+len(cr.okReplicas) > expectedShardSize
}

func (cr *shardRepair) hasReplicasToStart() bool {
	return len(cr.replicasToStart) > 0
}

func (cr *shardRepair) needToBeRestored() bool {
	if cr.available() || len(cr.replicasToStart) > 0 {
		return false
	}
	return true
}

func (cr *shardRepair) readyToBeRestored(mnh *multiNodeHost,
	currentTick uint64) ([]replica, bool) {
	if !cr.needToBeRestored() {
		return nil, false
	}
	nl := make([]replica, 0)
	for _, n := range cr.failedReplicas {
		spec, ok := mnh.Nodehosts[n.Address]
		if ok && spec.available(currentTick) && spec.hasLog(n.ShardID, n.ReplicaID) {
			nl = append(nl, n)
		}
	}
	return nl, len(cr.okReplicas)+len(nl) >= cr.quorum()
}

func (cr *shardRepair) canBeRestored(mnh *multiNodeHost,
	currentTick uint64) []replica {
	nl := make([]replica, 0)
	for _, n := range cr.failedReplicas {
		spec, ok := mnh.Nodehosts[n.Address]
		if ok && spec.available(currentTick) && spec.hasLog(n.ShardID, n.ReplicaID) {
			nl = append(nl, n)
		}
	}
	return nl
}

func (cr *shardRepair) logUnableToRestoreShard(mnh *multiNodeHost,
	currentTick uint64) {
	for _, n := range cr.failedReplicas {
		spec, ok := mnh.Nodehosts[n.Address]
		if ok {
			available := spec.available(currentTick)
			hasLog := spec.hasLog(n.ShardID, n.ReplicaID)
			plog.Debugf("node %s available: %t", n.Address, available)
			plog.Debugf("node %s hasLog for %s: %t", n.Address,
				logutil.DescribeNode(n.ShardID, n.ReplicaID), hasLog)
		} else {
			plog.Debugf("failed node %s not in mnh.nodehosts", n.Address)
		}
	}
}

// EntityFailed returns whether the timeline indicates that the entity has
// failed.
func EntityFailed(lastTick uint64, currentTick uint64) bool {
	return currentTick-lastTick > nodeHostTTL
}

func (n *replica) failed(tick uint64) bool {
	if n.Tick == 0 {
		return n.FirstObserved == 0
	}
	return EntityFailed(n.Tick, tick)
}

func (n *replica) id() string {
	return logutil.DescribeNode(n.ShardID, n.ReplicaID)
}

func (n *replica) waitingToBeStarted(tick uint64) bool {
	return n.Tick == 0 && !n.failed(tick)
}

func (c *shard) id() string {
	return logutil.ShardID(c.ShardID)
}

func (c *shard) quorum() int {
	return len(c.Replicas)/2 + 1
}

func (c *shard) available(currentTick uint64) bool {
	return len(c.getOkReplicas(currentTick)) >= c.quorum()
}

func (c *shard) getOkReplicas(tick uint64) []replica {
	result := make([]replica, 0)
	for _, n := range c.Replicas {
		if !n.failed(tick) && !n.waitingToBeStarted(tick) {
			result = append(result, *n)
		}
	}
	return result
}

func (c *shard) getReplicasToStart(tick uint64) []replica {
	result := make([]replica, 0)
	for _, n := range c.Replicas {
		if n.waitingToBeStarted(tick) {
			result = append(result, *n)
		}
	}
	return result
}

func (c *shard) getFailedReplicas(tick uint64) []replica {
	result := make([]replica, 0)
	for _, n := range c.Replicas {
		if n.failed(tick) {
			result = append(result, *n)
		}
	}
	return result
}

func (c *shard) deepCopy() *shard {
	nc := &shard{
		ShardID:           c.ShardID,
		ConfigChangeIndex: c.ConfigChangeIndex,
	}
	nc.Replicas = make(map[uint64]*replica)
	for k, v := range c.Replicas {
		nc.Replicas[k] = &replica{
			ShardID:       v.ShardID,
			ReplicaID:     v.ReplicaID,
			Address:       v.Address,
			IsLeader:      v.IsLeader,
			Tick:          v.Tick,
			FirstObserved: v.FirstObserved,
		}
	}
	return nc
}

func newMultiShard() *multiShard {
	return &multiShard{
		Shards: make(map[uint64]*shard),
	}
}

func (mc *multiShard) deepCopy() *multiShard {
	c := &multiShard{
		Shards: make(map[uint64]*shard),
	}
	for k, v := range mc.Shards {
		c.Shards[k] = v.deepCopy()
	}
	return c
}

func (mc *multiShard) getToKillReplicas() []replicaToKill {
	result := make([]replicaToKill, 0)
	for _, ntk := range mc.ReplicasToKill {
		n := replicaToKill{
			ShardID:   ntk.ShardID,
			ReplicaID: ntk.ReplicaID,
			Address:   ntk.Address,
		}
		result = append(result, n)
	}
	mc.ReplicasToKill = mc.ReplicasToKill[:0]
	return result
}

func (mc *multiShard) size() int {
	return len(mc.Shards)
}

func (mc *multiShard) getUnavailableShards(currentTick uint64) []shard {
	cl := make([]shard, 0)
	for _, c := range mc.Shards {
		if !c.available(currentTick) {
			cl = append(cl, *c)
		}
	}
	return cl
}

func (mc *multiShard) getShardForRepair(currentTick uint64) []shardRepair {
	crl := make([]shardRepair, 0)
	for _, c := range mc.Shards {
		failedReplicas := c.getFailedReplicas(currentTick)
		okReplicas := c.getOkReplicas(currentTick)
		toStartReplicas := c.getReplicasToStart(currentTick)
		if len(failedReplicas) == 0 && len(toStartReplicas) == 0 {
			continue
		}
		sort.Slice(failedReplicas, func(i, j int) bool {
			return failedReplicas[i].ShardID < failedReplicas[j].ShardID
		})
		sort.Slice(okReplicas, func(i, j int) bool {
			return okReplicas[i].ShardID < okReplicas[j].ShardID
		})
		sort.Slice(toStartReplicas, func(i, j int) bool {
			return toStartReplicas[i].ShardID < toStartReplicas[j].ShardID
		})
		cr := shardRepair{
			shard:           c,
			shardID:         c.ShardID,
			failedReplicas:  failedReplicas,
			okReplicas:      okReplicas,
			replicasToStart: toStartReplicas,
		}
		crl = append(crl, cr)
	}
	return crl
}

func (mc *multiShard) update(nhi pb.NodeHostInfo) {
	toKill := mc.doUpdate(nhi)
	for _, ntk := range toKill {
		n := replicaToKill{
			ShardID:   ntk.ShardId,
			ReplicaID: ntk.ReplicaId,
			Address:   nhi.RaftAddress,
		}
		mc.ReplicasToKill = append(mc.ReplicasToKill, n)
	}
	mc.syncLeaderInfo(nhi)
}

func (mc *multiShard) syncLeaderInfo(nhi pb.NodeHostInfo) {
	for _, ci := range nhi.ShardInfo {
		cid := ci.ShardId
		c, ok := mc.Shards[cid]
		if !ok {
			continue
		}
		if c.ConfigChangeIndex > ci.ConfigChangeIndex {
			continue
		}
		n, ok := c.Replicas[ci.ReplicaId]
		if !ok {
			continue
		}
		if !ci.IsLeader && n.IsLeader {
			n.IsLeader = false
		} else if ci.IsLeader && !n.IsLeader {
			for _, cn := range c.Replicas {
				cn.IsLeader = false
			}
			n.IsLeader = true
		}
	}
}

func (mc *multiShard) updateNodeTick(nhi pb.NodeHostInfo) {
	for _, shard := range nhi.ShardInfo {
		cid := shard.ShardId
		nid := shard.ReplicaId
		if ec, ok := mc.Shards[cid]; ok {
			if n, ok := ec.Replicas[nid]; ok {
				n.Tick = nhi.LastTick
			}
		}
	}
}

// doUpdate updates the local records of known shards using recently received
// NodeHostInfo received from nodehost, it returns a list of ShardInfo
// considered as zombie shards that are expected to be killed.
func (mc *multiShard) doUpdate(nhi pb.NodeHostInfo) []pb.ShardInfo {
	toKill := make([]pb.ShardInfo, 0)
	for _, currentShard := range nhi.ShardInfo {
		cid := currentShard.ShardId
		plog.Debugf("updating NodeHostInfo for %s, pending %t, incomplete %t",
			logutil.DescribeNode(currentShard.ShardId, currentShard.ReplicaId),
			currentShard.Pending, currentShard.Incomplete)
		if currentShard.Pending {
			if ec, ok := mc.Shards[cid]; ok {
				if len(ec.Replicas) > 0 && ec.ConfigChangeIndex > 0 &&
					ec.killRequestRequired(*currentShard) {
					toKill = append(toKill, *currentShard)
				}
			}
			continue
		}
		if !currentShard.Incomplete {
			if ec, ok := mc.Shards[cid]; !ok {
				// create not in the map, create it
				mc.Shards[cid] = mc.getShard(*currentShard, nhi.LastTick)
			} else {
				// shard record is there, sync the info
				rejected := ec.syncShard(*currentShard, nhi.LastTick)
				killRequired := ec.killRequestRequired(*currentShard)
				if rejected && killRequired {
					toKill = append(toKill, *currentShard)
				}
			}
		} else {
			// for this particular shard, we don't have the full info
			// check whether the shard/node are available, if true, update
			// the tick value
			ec, ca := mc.Shards[cid]
			if ca {
				if len(ec.Replicas) > 0 && ec.ConfigChangeIndex > 0 &&
					ec.killRequestRequired(*currentShard) {
					toKill = append(toKill, *currentShard)
				}
			}
		}
	}
	// update nodes tick
	mc.updateNodeTick(nhi)
	return toKill
}

func (mc *multiShard) GetShard(shardID uint64) *shard {
	return mc.getShardInfo(shardID)
}

func (c *shard) killRequestRequired(ci pb.ShardInfo) bool {
	if c.ConfigChangeIndex <= ci.ConfigChangeIndex {
		return false
	}
	for nid := range c.Replicas {
		if nid == ci.ReplicaId {
			return false
		}
	}
	return true
}

func (mc *multiShard) getShardInfo(shardID uint64) *shard {
	c, ok := mc.Shards[shardID]
	if !ok {
		return nil
	}
	return c.deepCopy()
}

func (mc *multiShard) getShard(ci pb.ShardInfo, tick uint64) *shard {
	c := &shard{
		ShardID:           ci.ShardId,
		ConfigChangeIndex: ci.ConfigChangeIndex,
		Replicas:          make(map[uint64]*replica),
	}
	for replicaID, address := range ci.Replicas {
		n := &replica{
			ReplicaID:     replicaID,
			Address:       address,
			ShardID:       ci.ShardId,
			FirstObserved: tick,
		}
		c.Replicas[replicaID] = n
	}
	n, ok := c.Replicas[ci.ReplicaId]
	if ok {
		n.IsLeader = ci.IsLeader
	}
	return c
}

func (c *shard) syncShard(ci pb.ShardInfo, lastTick uint64) bool {
	if c.ConfigChangeIndex > ci.ConfigChangeIndex {
		return true
	}
	if c.ConfigChangeIndex == ci.ConfigChangeIndex {
		if len(c.Replicas) != len(ci.Replicas) {
			plog.Panicf("same config change index with different node count")
		}
		for nid := range ci.Replicas {
			_, ok := c.Replicas[nid]
			if !ok {
				plog.Panicf("different node id list")
			}
		}
	}
	if c.ConfigChangeIndex < ci.ConfigChangeIndex {
		plog.Debugf("shard %s is being updated, %v, %v",
			c.id(), c.Replicas, ci)
	}
	// we don't just re-create and populate the Replicas map because the
	// FirstObserved value need to be kept after the sync.
	c.ConfigChangeIndex = ci.ConfigChangeIndex
	toRemove := make([]uint64, 0)
	for nid := range c.Replicas {
		_, ok := ci.Replicas[nid]
		if !ok {
			toRemove = append(toRemove, nid)
		}
	}
	for _, nid := range toRemove {
		plog.Debugf("shard %s is removing node %d due to config change %d",
			c.id(), nid, ci.ConfigChangeIndex)
		delete(c.Replicas, nid)
	}
	for replicaID, address := range ci.Replicas {
		if _, ok := c.Replicas[replicaID]; !ok {
			n := &replica{
				ReplicaID:     replicaID,
				Address:       address,
				ShardID:       ci.ShardId,
				FirstObserved: lastTick,
			}
			c.Replicas[replicaID] = n
			plog.Debugf("shard %s is adding node %d:%s due to config change %d",
				c.id(), replicaID, address, ci.ConfigChangeIndex)
		} else {
			n := c.Replicas[replicaID]
			if n.Address != address {
				plog.Panicf("changing node address in drummer")
			}
		}
	}
	addrMap := make(map[string]struct{})
	for _, n := range c.Replicas {
		_, ok := addrMap[n.Address]
		if ok {
			plog.Panicf("duplicated addr on drummer")
		} else {
			addrMap[n.Address] = struct{}{}
		}
	}
	return false
}
