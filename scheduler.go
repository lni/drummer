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

package drummer

import (
	"errors"

	pb "github.com/lni/drummer/v3/drummerpb"
	"github.com/lni/drummer/v3/settings"
	"github.com/lni/goutils/logutil"
	"github.com/lni/goutils/random"
)

var (
	// errNotEnoughNodeHost indicates that there are not enough node host
	// instances currently available.
	errNotEnoughNodeHost = errors.New("not enough node host")
	unknownRegion        = settings.Soft.UnknownRegionName
)

type scheduler struct {
	server         *server
	randomSrc      random.Source
	config         pb.Config
	shards         []*pb.Shard
	tick           uint64
	regions        *pb.Regions
	multiShard     *multiShard
	multiNodeHost  *multiNodeHost
	shardsToRepair []shardRepair
	nodeHostList   []*nodeHostSpec
	replicasToKill []replicaToKill
}

func newScheduler(server *server, config pb.Config) *scheduler {
	s := &scheduler{
		server:    server,
		config:    config,
		randomSrc: server.randSrc,
	}
	// set these redundant fields to be empty
	s.config.RaftShardAddresses = ""
	s.config.DrummerAddress = ""
	s.config.DrummerWALDirectory = ""
	s.config.DrummerNodeHostDirectory = ""
	return s
}

func newSchedulerWithContext(server *server,
	config pb.Config, tick uint64, shards []*pb.Shard,
	mc *multiShard, mnh *multiNodeHost) *scheduler {
	s := &scheduler{
		server:         server,
		config:         config,
		shards:         shards,
		tick:           tick,
		multiShard:     mc,
		multiNodeHost:  mnh,
		shardsToRepair: mc.getShardForRepair(tick),
		nodeHostList:   mnh.toArray(),
	}
	if server == nil {
		s.randomSrc = random.NewLockedRand()
	} else {
		s.randomSrc = server.randSrc
	}
	return s
}

//
// scheduler context
//

func (s *scheduler) updateSchedulerContext(sc *schedulerContext) {
	s.tick = sc.Tick
	s.shards = make([]*pb.Shard, 0)
	for _, c := range sc.Shards {
		s.shards = append(s.shards, c)
	}
	s.multiShard = sc.ShardImage
	s.multiNodeHost = sc.NodeHostImage
	s.regions = sc.Regions
	s.shardsToRepair = s.multiShard.getShardForRepair(s.tick)
	s.nodeHostList = s.multiNodeHost.toArray()
	s.replicasToKill = s.multiShard.getToKillReplicas()
}

func (s *scheduler) hasRunningShard() bool {
	return s.multiShard.size() > 0
}

//
// Launch shards related
//

func (s *scheduler) launch() ([]*pb.NodeHostRequest, error) {
	requests, err := s.getLaunchRequests(s.shards, s.regions)
	if err != nil {
		return nil, err
	}
	return requests, nil
}

func (s *scheduler) getLaunchRequests(shards []*pb.Shard,
	regions *pb.Regions) ([]*pb.NodeHostRequest, error) {
	result := make([]*pb.NodeHostRequest, 0)
	plog.Infof("hosts available for launch requests:")
	for _, nh := range s.nodeHostList {
		plog.Infof("address %s, region %s", nh.Address, nh.Region)
	}
	plog.Infof("regions content %v", regions)
	for _, shard := range shards {
		selected := make([]*nodeHostSpec, 0)
		for idx, reg := range regions.Region {
			cnt := int(regions.Count[idx])
			randSelector := newRandomRegionSelector(reg,
				shard.ShardId, s.tick, nodeHostTTL, s.randomSrc)
			regionReplicas := randSelector.findSuitableNodeHost(s.nodeHostList, cnt)
			if len(regionReplicas) != cnt {
				plog.Errorf("failed to get enough node host for shard %d region %s",
					shard.ShardId, reg)
			}
			selected = append(selected, regionReplicas...)
		}
		// don't have enough suitable replicas
		if len(selected) < len(shard.Members) {
			// FIXME: check whether this setup is actually aborted.
			return nil, errors.New("not enough nodehost in suitable regions")
		}
		replicaIDList := make([]uint64, 0)
		addressList := make([]string, 0)
		for idx, m := range shard.Members {
			replicaIDList = append(replicaIDList, m)
			addressList = append(addressList, selected[idx].Address)
		}
		change := &pb.Request{
			Type:    pb.Request_CREATE,
			ShardId: shard.ShardId,
			Members: replicaIDList,
		}
		cfg := s.config
		for idx, targetNode := range selected {
			req := &pb.NodeHostRequest{
				Change:               change,
				ReplicaIdList:        replicaIDList,
				AddressList:          addressList,
				InstantiateReplicaId: shard.Members[idx],
				RaftAddress:          targetNode.Address,
				Join:                 false,
				Restore:              false,
				AppName:              shard.AppName,
				Config:               &cfg,
			}
			result = append(result, req)
		}
	}
	return result, nil
}

//
// Repair shards related
//

func (s *scheduler) repair(restored map[uint64]struct{}) ([]*pb.NodeHostRequest, error) {
	plog.Infof("toRepair sz: %d", len(s.shardsToRepair))
	result := make([]*pb.NodeHostRequest, 0)
	for _, cc := range s.shardsToRepair {
		_, ok := restored[cc.shardID]
		if ok {
			plog.Infof("shard %s is being restored, skipping repair task",
				cc.id())
			continue
		}
		plog.Infof("drummer shard status %s, ok %d failed %d waiting %d",
			cc.id(), len(cc.okReplicas),
			len(cc.failedReplicas), len(cc.replicasToStart))
		expectedShardSize := s.getShardSize(cc.shardID)
		plog.Infof("delete required %t, create required %t, add required %t",
			cc.deleteRequired(expectedShardSize),
			cc.createRequired(), cc.addRequired())
		if cc.deleteRequired(expectedShardSize) {
			toDeleteNode := cc.failedReplicas[0]
			reqs, err := s.getRepairDeleteRequest(toDeleteNode, cc)
			if err != nil {
				return nil, err
			}
			plog.Infof("scheduler generated a delete request for %s on %s",
				toDeleteNode.id(), toDeleteNode.Address)
			result = append(result, reqs...)
		} else if cc.createRequired() {
			toCreateNode := cc.replicasToStart[0]
			appName := s.getAppName(cc.shardID)
			reqs, err := s.getRepairCreateRequest(toCreateNode, *cc.shard, appName)
			if err != nil {
				return nil, err
			}
			plog.Infof("scheduler generated a create request for %s on %s",
				toCreateNode.id(), toCreateNode.Address)
			result = append(result, reqs...)
		} else if cc.addRequired() {
			failedReplica := cc.failedReplicas[0]
			reqs, err := s.getRepairAddRequest(failedReplica, cc)
			if err != nil {
				return nil, err
			}
			plog.Infof("scheduler generated a add request for failed node %s on %s,"+
				"new node id %d, new node address %s, target nodehost %s",
				failedReplica.id(),
				failedReplica.Address,
				reqs[0].Change.Members[0],
				reqs[0].AddressList[0],
				reqs[0].RaftAddress)
			result = append(result, reqs...)
		}
	}
	return result, nil
}

// for an identified failed node, get an ADD request to add a new node
func (s *scheduler) getRepairAddRequest(failedReplica replica,
	ctr shardRepair) ([]*pb.NodeHostRequest, error) {
	selected := s.getReplacementNode(failedReplica)
	if len(selected) != 1 {
		plog.Warningf("failed to find a replacement node for the failed node %s",
			failedReplica.id())
		return nil, errNotEnoughNodeHost
	}
	selectedNode := selected[0]
	okNodeCount := len(ctr.okReplicas)
	existingMemberAddress := ctr.okReplicas[s.randomSrc.Int()%okNodeCount].Address
	newReplicaID := s.randomSrc.Uint64()
	change := &pb.Request{
		Type:         pb.Request_ADD,
		ShardId:      ctr.shardID,
		Members:      []uint64{newReplicaID},
		ConfChangeId: ctr.shard.ConfigChangeIndex,
	}
	req := &pb.NodeHostRequest{
		Change:      change,
		RaftAddress: existingMemberAddress,
		AddressList: []string{selectedNode.Address},
	}
	return []*pb.NodeHostRequest{req}, nil
}

func (s *scheduler) getReplacementNode(failedReplica replica) []*nodeHostSpec {
	// see whether we can find one in the same region
	var region string
	nodeHostSpec, ok := s.multiNodeHost.Nodehosts[failedReplica.Address]
	if !ok {
		plog.Errorf("failed to get region")
		region = unknownRegion
	} else {
		region = nodeHostSpec.Region
	}
	regionSelector := newRandomRegionSelector(region,
		failedReplica.ShardID, s.tick, nodeHostTTL, s.randomSrc)
	selected := regionSelector.findSuitableNodeHost(s.nodeHostList, 1)
	if len(selected) == 1 {
		return selected
	}
	plog.Warningf("failed to find a nodehost in region %s", region)
	// get a random one
	randSelector := newRandomSelector(failedReplica.ShardID,
		s.tick, nodeHostTTL, s.randomSrc)
	return randSelector.findSuitableNodeHost(s.nodeHostList, 1)
}

// start the node which was previously added to the raft shard
func (s *scheduler) getRepairCreateRequest(newReplica replica,
	ci shard, appName string) ([]*pb.NodeHostRequest, error) {
	return s.getCreateRequest(newReplica, ci, appName, true, false)
}

// get the request to delete the specified node from the raft shard
func (s *scheduler) getRepairDeleteRequest(replicaToDelete replica,
	ctr shardRepair) ([]*pb.NodeHostRequest, error) {
	okReplicaCount := len(ctr.okReplicas)
	existingMemberAddress := ctr.okReplicas[s.randomSrc.Int()%okReplicaCount].Address
	change := &pb.Request{
		Type:         pb.Request_DELETE,
		ShardId:      ctr.shardID,
		Members:      []uint64{replicaToDelete.ReplicaID},
		ConfChangeId: ctr.shard.ConfigChangeIndex,
	}
	req := &pb.NodeHostRequest{
		Change:      change,
		RaftAddress: existingMemberAddress,
	}
	return []*pb.NodeHostRequest{req}, nil
}

// there are multiple reasons why we need such a local kill request to
// remove the specified node from the nodehost on the receiving end -
//  1. in the current raft implementation, there is no guarantee that
//     the node being deleted will be removed from its hosting nodehost.
//     consider the situation in which the remote object for the node being
//     deleted is removed from the leader before the delete config change
//     entry can be replicated onto the node being deleted.
//  2. drummer CREATE requests can be sitting in the pipeline for a while
//     when the selected nodehost to start this new node is down. drummer
//     might delete the node before the nodehost picks up the CREATE request
//     and start the node.
//
// in both of these two situations, the concerned replicas becomes so called
// zombie, it is not going to cause real trouble for the raft shard,
// but having it hanging around is not that desirable.
func (s *scheduler) killZombieReplicas() []*pb.NodeHostRequest {
	results := make([]*pb.NodeHostRequest, 0)
	for _, ntk := range s.replicasToKill {
		req := s.getKillRequest(ntk.ShardID, ntk.ReplicaID, ntk.Address)
		results = append(results, req)
		plog.Infof("scheduler generated a kill request for %s on %s",
			logutil.DescribeNode(ntk.ShardID, ntk.ReplicaID), ntk.Address)
	}
	return results
}

func (s *scheduler) getKillRequest(shardID uint64,
	replicaID uint64, addr string) *pb.NodeHostRequest {
	kc := &pb.Request{
		Type:    pb.Request_KILL,
		ShardId: shardID,
		Members: []uint64{replicaID},
	}
	return &pb.NodeHostRequest{
		Change:      kc,
		RaftAddress: addr,
	}
}

func (s *scheduler) getCreateRequest(newReplica replica,
	ci shard, appName string, join bool,
	restore bool) ([]*pb.NodeHostRequest, error) {
	replicaIDList := make([]uint64, 0)
	addressList := make([]string, 0)
	for _, n := range ci.Replicas {
		replicaIDList = append(replicaIDList, n.ReplicaID)
		addressList = append(addressList, n.Address)
	}
	change := &pb.Request{
		Type:    pb.Request_CREATE,
		ShardId: ci.ShardID,
		Members: replicaIDList,
	}
	cfg := s.config
	req := &pb.NodeHostRequest{
		Change:               change,
		ReplicaIdList:        replicaIDList,
		AddressList:          addressList,
		InstantiateReplicaId: newReplica.ReplicaID,
		RaftAddress:          newReplica.Address,
		Join:                 join,
		Restore:              restore,
		AppName:              appName,
		Config:               &cfg,
	}
	return []*pb.NodeHostRequest{req}, nil
}

// Restore unavailable shards related
func (s *scheduler) restore() ([]*pb.NodeHostRequest, error) {
	ureqs, done, err := s.restoreUnavailableShards()
	if err != nil {
		return nil, err
	}
	reqs, err := s.restoreFailed(done)
	if err != nil {
		return nil, err
	}
	return append(ureqs, reqs...), nil
}

func (s *scheduler) restoreUnavailableShards() ([]*pb.NodeHostRequest,
	map[uint64]struct{}, error) {
	result := make([]*pb.NodeHostRequest, 0)
	idMap := make(map[uint64]struct{})
	for _, cc := range s.shardsToRepair {
		if !cc.needToBeRestored() {
			continue
		}
		replicasToRestore, ok := cc.readyToBeRestored(s.multiNodeHost, s.tick)
		if ok {
			plog.Infof("shard %s is unavailable but can be restored (%d)",
				cc.id(), len(replicasToRestore))
			for _, toRestoreNode := range replicasToRestore {
				appName := s.getAppName(cc.shardID)
				reqs, err := s.getRestoreCreateRequest(toRestoreNode,
					*cc.shard, appName)
				if err != nil {
					plog.Warningf("failed to get restore request %v", err)
					return nil, nil, err
				}
				plog.Infof("scheduler created a restore requeset for %s on %s",
					toRestoreNode.id(), toRestoreNode.Address)
				result = append(result, reqs...)
				idMap[cc.shardID] = struct{}{}
			}
		} else {
			plog.Warningf("shard %s is unavailable and can not be restored",
				cc.id())
			cc.logUnableToRestoreShard(s.multiNodeHost, s.tick)
		}
	}
	return result, idMap, nil
}

func (s *scheduler) restoreFailed(done map[uint64]struct{}) ([]*pb.NodeHostRequest,
	error) {
	result := make([]*pb.NodeHostRequest, 0)
	for _, cc := range s.shardsToRepair {
		_, ok := done[cc.shardID]
		if cc.needToBeRestored() || ok {
			continue
		}
		replicasToRestore := cc.canBeRestored(s.multiNodeHost, s.tick)
		plog.Infof("shard %s is unavailable but can be restored (%d)",
			cc.id(), len(replicasToRestore))
		for _, toRestoreNode := range replicasToRestore {
			appName := s.getAppName(cc.shardID)
			reqs, err := s.getRestoreCreateRequest(toRestoreNode,
				*cc.shard, appName)
			if err != nil {
				return nil, err
			}
			plog.Infof("scheduler created a restore requeset for %s on %s",
				toRestoreNode.id(), toRestoreNode.Address)
			result = append(result, reqs...)
		}
	}
	if len(result) > 0 {
		plog.Infof("trying to restore %d failed replicas", len(result))
	}
	return result, nil
}

func (s *scheduler) getRestoreCreateRequest(newReplica replica,
	ci shard, appName string) ([]*pb.NodeHostRequest, error) {
	return s.getCreateRequest(newReplica, ci, appName, false, true)
}

//
// helper functions
//

func (s *scheduler) getShardSize(shardID uint64) int {
	for _, c := range s.shards {
		if c.ShardId == shardID {
			return len(c.Members)
		}
	}
	panic("failed to locate the shard")
}

func (s *scheduler) getAppName(shardID uint64) string {
	for _, c := range s.shards {
		if c.ShardId == shardID {
			return c.AppName
		}
	}
	panic("failed to locate the shard")
}
