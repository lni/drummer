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

package client

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/raftio"
	pb "github.com/lni/drummer/v3/drummerpb"
	"github.com/lni/drummer/v3/settings"
	"github.com/lni/goutils/logutil"
	"github.com/lni/goutils/syncutil"
)

const (
	// DefaultRegion is the default region name used in drummer monkey tests
	DefaultRegion string = "default-region"
)

var (
	// HardWorkerTestShardID is the cluster ID of the cluster targeted by the
	// hard worker.
	HardWorkerTestShardID uint64 = 1
	// DrummerClientName is the name of the default master client.
	DrummerClientName      = settings.Soft.DrummerClientName
	getConnectedTimeoutSec = settings.Soft.GetConnectedTimeoutSecond
	// localTimeoutMs is the timeout in millisecond value to use when ininating
	// Raft requests locally on nodehost.
	localTimeoutMs = time.Duration(settings.Soft.LocalRaftRequestTimeoutMs) * time.Millisecond
)

func isGRPCTempError(err error) bool {
	return status.Code(err) == codes.Unavailable
}

// DrummerClient is the client used to contact drummer servers.
type DrummerClient struct {
	nh  *dragonboat.NodeHost
	req struct {
		sync.Mutex
		requests []*pb.NodeHostRequest
	}
	mu struct {
		sync.Mutex
		// app name -> func(uint64, uint64) IStateMachine
		smFactory map[string]pluginDetails
	}
	connections *Pool
}

// NewDrummerClient creates a new drummer client instance.
func NewDrummerClient(nh *dragonboat.NodeHost) *DrummerClient {
	dc := &DrummerClient{
		nh:          nh,
		connections: NewDrummerConnectionPool(),
	}
	dc.req.requests = make([]*pb.NodeHostRequest, 0)
	// currently it is hard coded to scan the working dir for plugins.
	// might need to make this configurable.
	dc.mu.smFactory = getPluginMap(".")
	return dc
}

// Name returns the name of the drummer client.
func (dc *DrummerClient) Name() string {
	return DrummerClientName
}

// Stop stops the drummer client.
func (dc *DrummerClient) Stop() {
	dc.connections.Close()
}

type clusterInfo struct {
	info       dragonboat.ShardInfo
	incomplete bool
}

// SendNodeHostInfo send the node host info the specified drummer server.
func (dc *DrummerClient) SendNodeHostInfo(ctx context.Context,
	drummerAPIAddress string,
	nhi dragonboat.NodeHostInfo,
	APIAddress string, logInfoIncluded bool) error {
	if IsNodeHostPartitioned(dc.nh) {
		plog.Infof("in partitioned test mode, dropping NodeHost Info report msg")
		return nil
	}
	conn, err := dc.getDrummerConnection(ctx, drummerAPIAddress)
	if err != nil {
		return err
	}
	client := pb.NewDrummerClient(conn.ClientConn())
	il, err := client.GetShardConfigChangeIndexList(ctx, nil)
	if err != nil {
		conn.Close()
		return err
	}
	plog.Debugf("%s got cluster config change index from %s: %v",
		nhi.RaftAddress, drummerAPIAddress, il.Indexes)
	cil := make([]clusterInfo, 0)
	shardIDList := make([]uint64, 0)
	for _, v := range nhi.ShardInfoList {
		incomplete := false
		shardIDList = append(shardIDList, v.ShardID)
		cci, ok := il.Indexes[v.ShardID]
		if ok && cci >= v.ConfigChangeIndex && !v.Pending {
			incomplete = true
			v.Replicas = nil
		}
		ci := clusterInfo{info: v, incomplete: incomplete}
		cil = append(cil, ci)
	}
	for _, v := range cil {
		info := v.info
		if !v.incomplete && !info.Pending && len(info.Replicas) > 0 {
			plog.Debugf("%s updating nodehost info, %d, %v",
				nhi.RaftAddress, info.ConfigChangeIndex, info.Replicas)
		}
	}
	if !logInfoIncluded && len(nhi.LogInfo) != 0 {
		panic("!plogIncluded but len(logInfo) != 0")
	}
	info := &pb.NodeHostInfo{
		RaftAddress:      nhi.RaftAddress,
		RPCAddress:       APIAddress,
		ShardInfo:        toDrummerPBShardInfo(cil),
		ShardIdList:      shardIDList,
		PlogInfoIncluded: logInfoIncluded,
		PlogInfo:         toDrummerPBLogInfo(nhi.LogInfo),
		Region:           DefaultRegion,
	}
	requestCollection, err := client.ReportAvailableNodeHost(ctx, info)
	if err != nil {
		plog.Warningf("failed to report nodehost info to %s, %s",
			drummerAPIAddress, err)
		conn.Close()
		return err
	}
	plog.Infof("%s received %d nodehost requests from %s",
		dc.nh.RaftAddress(),
		len(requestCollection.Requests), drummerAPIAddress)
	if len(requestCollection.Requests) > 0 {
		dc.addRequests(requestCollection.Requests)
	}
	return nil
}

func toDrummerPBShardInfo(cil []clusterInfo) []*pb.ShardInfo {
	result := make([]*pb.ShardInfo, 0)
	for _, vv := range cil {
		v := vv.info
		incomplete := vv.incomplete
		pbv := &pb.ShardInfo{
			ShardId:           v.ShardID,
			ReplicaId:         v.ReplicaID,
			IsLeader:          v.IsLeader,
			Replicas:          v.Replicas,
			ConfigChangeIndex: v.ConfigChangeIndex,
			Incomplete:        incomplete,
			Pending:           v.Pending,
		}
		result = append(result, pbv)
	}
	return result
}

func toDrummerPBLogInfo(loginfo []raftio.NodeInfo) []*pb.LogInfo {
	result := make([]*pb.LogInfo, 0)
	for _, v := range loginfo {
		pbv := &pb.LogInfo{
			ShardId:   v.ShardID,
			ReplicaId: v.ReplicaID,
		}
		result = append(result, pbv)
	}
	return result
}

// HandleMasterRequests handles requests made by master servers.
func (dc *DrummerClient) HandleMasterRequests(ctx context.Context) error {
	reqs := dc.getRequests()
	if len(reqs) == 0 {
		return nil
	}
	shardIDMap := make(map[uint64]struct{})
	for _, req := range reqs {
		cid := req.Change.ShardId
		shardIDMap[cid] = struct{}{}
	}
	// each cluster is handled in its own worker goroutine
	stopper := syncutil.NewStopper()
	for k := range shardIDMap {
		shardID := k
		stopper.RunWorker(func() {
			for _, req := range reqs {
				if req.Change.ShardId == shardID {
					dc.handleRequest(ctx, req)
				}
			}
		})
	}
	stopper.Stop()
	return nil
}

func (dc *DrummerClient) getDrummerConnection(ctx context.Context,
	drummerAPIAddress string) (*Connection, error) {
	getConnTimeout := time.Duration(getConnectedTimeoutSec) * time.Second
	getConnCtx, cancel := context.WithTimeout(ctx, getConnTimeout)
	defer cancel()
	nhCfg := dc.nh.NodeHostConfig()
	tlsConfig, err := nhCfg.GetClientTLSConfig(drummerAPIAddress)
	if err != nil {
		return nil, err
	}
	conn, err := dc.connections.GetConnection(getConnCtx,
		drummerAPIAddress, tlsConfig)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (dc *DrummerClient) addRequests(reqs []*pb.NodeHostRequest) {
	dc.req.Lock()
	defer dc.req.Unlock()
	dc.req.requests = append(dc.req.requests, reqs...)
}

func (dc *DrummerClient) getRequests() []*pb.NodeHostRequest {
	dc.req.Lock()
	defer dc.req.Unlock()
	results := dc.req.requests
	dc.req.requests = make([]*pb.NodeHostRequest, 0)
	return results
}

func (dc *DrummerClient) handleRequest(ctx context.Context,
	req *pb.NodeHostRequest) {
	reqCtx, cancel := context.WithTimeout(ctx, localTimeoutMs)
	defer cancel()
	if req.Change.Type == pb.Request_CREATE {
		dc.handleInstantiateRequest(req)
	} else if req.Change.Type == pb.Request_DELETE ||
		req.Change.Type == pb.Request_ADD {
		dc.handleAddDeleteRequest(reqCtx, req)
	} else if req.Change.Type == pb.Request_KILL {
		dc.handleKillRequest(req)
	} else {
		panic("unknown request type")
	}
}

func (dc *DrummerClient) handleKillRequest(req *pb.NodeHostRequest) {
	replicaID := req.Change.Members[0]
	shardID := req.Change.ShardId
	plog.Debugf("kill request handled on %s for %s",
		dc.nh.RaftAddress(), logutil.DescribeNode(shardID, replicaID))
	if err := dc.nh.StopReplica(shardID, replicaID); err != nil {
		plog.Errorf("removeShardNode for %s failed, %v",
			logutil.DescribeNode(shardID, replicaID), err)
	} else {
		plog.Infof("going to remove data for %s",
			logutil.DescribeNode(shardID, replicaID))
		for {
			err := dc.nh.RemoveData(shardID, replicaID)
			if err != nil {
				plog.Errorf("remove data failed %s, %v",
					logutil.DescribeNode(shardID, replicaID), err)
				if err == dragonboat.ErrShardNotStopped {
					time.Sleep(100 * time.Millisecond)
				} else {
					panic(err)
				}
			} else {
				break
			}
		}
	}
}

func (dc *DrummerClient) handleAddDeleteRequest(ctx context.Context,
	req *pb.NodeHostRequest) {
	replicaID := req.Change.Members[0]
	shardID := req.Change.ShardId
	var rs *dragonboat.RequestState
	var err error
	if req.Change.Type == pb.Request_DELETE {
		plog.Infof("delete request handled on %s for %s, conf change id %d",
			dc.nh.RaftAddress(), logutil.DescribeNode(shardID, replicaID),
			req.Change.ConfChangeId)
		rs, err = dc.nh.RequestDeleteReplica(shardID, replicaID,
			req.Change.ConfChangeId, localTimeoutMs)
	} else if req.Change.Type == pb.Request_ADD {
		plog.Infof("add request handled on %s for %s, conf change id %d",
			dc.nh.RaftAddress(), logutil.DescribeNode(shardID, replicaID),
			req.Change.ConfChangeId)
		url := req.AddressList[0]
		rs, err = dc.nh.RequestAddReplica(shardID, replicaID, url,
			req.Change.ConfChangeId, localTimeoutMs)
	} else {
		plog.Panicf("unknown request type %s", req.Change.Type)
	}
	if err == dragonboat.ErrShardNotFound ||
		dragonboat.IsTempError(err) ||
		isGRPCTempError(err) {
		return
	} else if err != nil {
		panic(err)
	}
	select {
	case <-ctx.Done():
		return
	case v := <-rs.CompletedC:
		if !v.Completed() &&
			!v.Timeout() &&
			!v.Terminated() &&
			!v.Rejected() &&
			!v.Dropped() {
			plog.Panicf("unknown result code: %v", v)
		}
		if v.Completed() && req.Change.Type == pb.Request_DELETE {
			plog.Infof("DELETE node completed, try to remove data for %s",
				logutil.DescribeNode(shardID, replicaID))
			for {
				err := dc.nh.RemoveData(shardID, replicaID)
				if err != nil {
					plog.Errorf("remove deleted node's data failed %s, %v",
						logutil.DescribeNode(shardID, replicaID), err)
					if err == dragonboat.ErrShardNotStopped {
						time.Sleep(100 * time.Millisecond)
					} else {
						panic(err)
					}
				} else {
					break
				}
			}
		}
		return
	}
}

func getConfig(req *pb.NodeHostRequest) config.Config {
	return config.Config{
		DisableAutoCompactions: true,
		ElectionRTT:            req.Config.ElectionRTT,
		HeartbeatRTT:           req.Config.HeartbeatRTT,
		CheckQuorum:            req.Config.CheckQuorum,
		SnapshotEntries:        req.Config.SnapshotEntries,
		CompactionOverhead:     req.Config.CompactionOverhead,
		MaxInMemLogSize:        req.Config.MaxInMemLogSize,
	}
}

func (dc *DrummerClient) handleInstantiateRequest(req *pb.NodeHostRequest) {
	requestType := ""
	replicaID := req.InstantiateReplicaId
	shardID := req.Change.ShardId
	hasNodeInfo := dc.nh.HasNodeInfo(shardID, replicaID)
	peers := make(map[uint64]string)
	// based on the request, check whether the local NodeInfo record is consistent
	// with what drummer wants us to do
	if req.Join && !req.Restore {
		// repair
		requestType = "join"
		if hasNodeInfo {
			plog.Warningf("node %s info found on %s, will ignore the request",
				logutil.DescribeNode(shardID, replicaID), dc.nh.RaftAddress())
		}
	} else if !req.Join && req.Restore {
		// restore
		requestType = "restore"
		if !hasNodeInfo {
			plog.Warningf("node %s info not found on %s, disk has been replaced?",
				logutil.DescribeNode(shardID, replicaID), dc.nh.RaftAddress())
			return
		}
	} else if !req.Join && !req.Restore {
		// launch
		requestType = "launch"
		if hasNodeInfo {
			plog.Panicf("node %s info found on %s, launch failed",
				logutil.DescribeNode(shardID, replicaID), dc.nh.RaftAddress())
		}
		for k, v := range req.AddressList {
			plog.Debugf("remote node info - id:%s, address:%s",
				logutil.ReplicaID(req.ReplicaIdList[k]), v)
			peers[req.ReplicaIdList[k]] = v
		}
	} else {
		panic("unknown join && restore combination")
	}
	plog.Infof("%s request handled on %s for %s",
		requestType, dc.nh.RaftAddress(),
		logutil.DescribeNode(shardID, replicaID))
	config := getConfig(req)
	config.ReplicaID = replicaID
	config.ShardID = req.Change.ShardId
	config.OrderedConfigChange = true
	pd, ok := dc.mu.smFactory[req.AppName]
	if !ok {
		// installation or configuration issue
		panic("failed to start the node as the plugin is not ready")
	}
	var err error
	if pd.isRegularStateMachine() {
		err = dc.nh.StartReplica(peers,
			req.Join, pd.createNativeStateMachine, config)
	} else if pd.isConcurrentStateMachine() {
		err = dc.nh.StartConcurrentReplica(peers,
			req.Join, pd.createConcurrentStateMachine, config)
	} else if pd.isOnDiskStateMachine() {
		err = dc.nh.StartOnDiskReplica(peers,
			req.Join, pd.createOnDiskStateMachine, config)
	} else {
		panic("unknown type")
	}
	if err != nil {
		plog.Errorf("add cluster %s failed: %v",
			logutil.DescribeNode(shardID, replicaID), err)
	}
}
