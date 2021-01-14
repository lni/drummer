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

// +build dragonboat_monkeytest

package drummer

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lni/goutils/logutil"
	"github.com/lni/goutils/random"
	"github.com/lni/goutils/syncutil"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/raftpb"
	"github.com/lni/drummer/v3/client"
	pb "github.com/lni/drummer/v3/drummerpb"
	"github.com/lni/drummer/v3/kv"
	"github.com/lni/drummer/v3/lcm"
	mr "github.com/lni/drummer/v3/multiraftpb"
)

const (
	defaultBasePort    uint64 = 5700
	defaultNodeID1     uint64 = 2345
	defaultNodeID2     uint64 = 6789
	defaultNodeID3     uint64 = 9876
	defaultTestTimeout        = 5 * time.Second
	lcmlog                    = "drummer-lcm.jepsen"
	ednlog                    = "drummer-lcm.edn"
)

var dn = logutil.DescribeNode

type nodeType uint64

func saveHeapProfile(fn string) {
	if mf, err := os.Create(fn); err != nil {
		panic(err)
	} else {
		defer mf.Close()
		pprof.WriteHeapProfile(mf)
	}
}

const (
	monkeyTestWorkingDir          = "drummer_mt_pwd_safe_to_delete"
	nodeTypeDrummer      nodeType = iota
	nodeTypeNodehost
)

func (t nodeType) String() string {
	if t == nodeTypeDrummer {
		return "DrummerNode"
	} else if t == nodeTypeNodehost {
		return "NodehostNode"
	} else {
		panic("unknown type")
	}
}

func getEntryListHash(entries []raftpb.Entry) uint64 {
	h := md5.New()
	v := make([]byte, 8)
	for _, ent := range entries {
		binary.LittleEndian.PutUint64(v, ent.Index)
		if _, err := h.Write(v); err != nil {
			panic(err)
		}
		binary.LittleEndian.PutUint64(v, ent.Term)
		if _, err := h.Write(v); err != nil {
			panic(err)
		}
		binary.LittleEndian.PutUint64(v, uint64(ent.Type))
		if _, err := h.Write(v); err != nil {
			panic(err)
		}
		if _, err := h.Write(ent.Cmd); err != nil {
			panic(err)
		}
	}
	return binary.LittleEndian.Uint64(h.Sum(nil)[:8])
}

func getEntryHash(ent raftpb.Entry) uint64 {
	h := md5.New()
	_, err := h.Write(ent.Cmd)
	if err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(h.Sum(nil)[:8])
}

func getConfigFromJSON() (config.Config, bool) {
	cfg := config.Config{}
	fn := "dragonboat-drummer.json"
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		return config.Config{}, false
	}
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		panic(err)
	}
	return cfg, true
}

func rateLimiterDisabledInConfig() bool {
	cfg, ok := getConfigFromJSON()
	if !ok {
		return true
	}
	return cfg.MaxInMemLogSize == 0
}

func snapshotDisabledInConfig() bool {
	cfg, ok := getConfigFromJSON()
	if !ok {
		return false
	}
	return cfg.SnapshotEntries == 0
}

func lessSnapshotTest() bool {
	cfg, ok := getConfigFromJSON()
	if !ok {
		return false
	}
	return cfg.SnapshotEntries > 30
}

func printEntries(clusterID uint64, nodeID uint64, entries []raftpb.Entry) {
	for _, ent := range entries {
		plog.Infof("%s, idx %d, term %d, type %s, entry len %d, hash %d",
			dn(clusterID, nodeID), ent.Index, ent.Term, ent.Type,
			len(ent.Cmd), getEntryHash(ent))
	}
}

func logCluster(nodes []*testNode, clusterIDMap map[uint64]struct{}) {
	for _, n := range nodes {
		nh := n.nh
		for _, rn := range nh.Clusters() {
			clusterID := rn.ClusterID()
			if _, ok := clusterIDMap[clusterID]; ok {
				plog.Infof("%s rn.lastApplied %d",
					dn(rn.ClusterID(), rn.NodeID()), rn.GetLastApplied())
				rn.DumpRaftInfoToLog()
			}
		}
	}
}

func logClusterToRepair(cl []clusterRepair, tick uint64) {
	plog.Infof("cluster to repair info, tick %d", tick)
	for _, c := range cl {
		plog.Infof("cluster id %d, cfg chg idx %d, failed %v, ok %v, to start %v",
			c.clusterID, c.cluster.ConfigChangeIndex,
			c.failedNodes, c.okNodes, c.nodesToStart)
	}
}

func logUnavailableCluster(cl []cluster, tick uint64) {
	plog.Infof("unavailable cluster info, tick %d", tick)
	for _, c := range cl {
		plog.Infof("cluster id %d, config change idx %d, nodes %v",
			c.ClusterID, c.ConfigChangeIndex, c.Nodes)
	}
}

func disableRandomDelay() {
	if err := os.Setenv("IOEI", "disabled"); err != nil {
		panic(err)
	}
}

func disableClusterRandomDelay(clusterID uint64) {
	pcs := fmt.Sprintf("IOEI-%d", clusterID)
	if err := os.Setenv(pcs, "disabled"); err != nil {
		panic(err)
	}
}

func getRandomClusterID(size uint64) uint64 {
	return (rand.Uint64() % size) + 1
}

type testSetup struct {
	snapshotWorkerCount       uint64
	applyWorkerCount          uint64
	monkeyTestSecondToRun     uint64
	numOfClusters             uint64
	numOfTestDrummerNodes     uint64
	numOfTestNodeHostNodes    uint64
	LCMWorkerCount            uint64
	testClientWorkerCount     uint64
	partitionCycle            uint64
	partitionMinSecond        uint64
	partitionMinStartSecond   uint64
	partitionCycleInterval    uint64
	partitionCycleMinInterval uint64
	waitForStableSecond       uint64
	testIdleTime              uint64
	nodeUpTimeLowSecond       uint64
	nodeUpTimeHighSecond      uint64
	maxWaitForStopSecond      uint64
	maxWaitForSyncSecond      uint64
	maxAllowedHeapSize        uint64
	drummerAddrs              []string
	nodehostAddrs             []string
	drummerAPIAddrs           []string
	nodehostAPIAddrs          []string
	drummerDirs               []string
	nodehostDirs              []string
	slowvm                    bool
}

func newTestSetup(to *testOption) *testSetup {
	port := to.port
	ts := &testSetup{
		drummerAddrs:              make([]string, 0),
		nodehostAddrs:             make([]string, 0),
		drummerAPIAddrs:           make([]string, 0),
		nodehostAPIAddrs:          make([]string, 0),
		drummerDirs:               make([]string, 0),
		nodehostDirs:              make([]string, 0),
		snapshotWorkerCount:       to.snapshotWorkerCount,
		applyWorkerCount:          to.workerCount,
		monkeyTestSecondToRun:     1200,
		numOfClusters:             128,
		numOfTestDrummerNodes:     3,
		numOfTestNodeHostNodes:    5,
		LCMWorkerCount:            32,
		testClientWorkerCount:     32,
		partitionCycle:            60,
		partitionMinSecond:        20,
		partitionMinStartSecond:   200,
		partitionCycleInterval:    60,
		partitionCycleMinInterval: 30,
		waitForStableSecond:       25,
		testIdleTime:              30,
		nodeUpTimeLowSecond:       150000,
		nodeUpTimeHighSecond:      240000,
		maxWaitForStopSecond:      60,
		maxWaitForSyncSecond:      120,
		maxAllowedHeapSize:        1024 * 1024 * 1024 * 4,
		slowvm:                    to.slowvm,
	}
	port = port + 1
	for i := uint64(0); i < uint64(ts.numOfTestDrummerNodes); i++ {
		addr := fmt.Sprintf("localhost:%d", port)
		ts.drummerAddrs = append(ts.drummerAddrs, addr)
		nn := fmt.Sprintf("drummer-node-%d", i)
		ts.drummerDirs = append(ts.drummerDirs, nn)
		port++
	}
	for i := uint64(0); i < uint64(ts.numOfTestNodeHostNodes); i++ {
		addr := fmt.Sprintf("localhost:%d", port)
		ts.nodehostAddrs = append(ts.nodehostAddrs, addr)
		nn := fmt.Sprintf("nodehost-node-%d", i)
		ts.nodehostDirs = append(ts.nodehostDirs, nn)
		port++
	}
	for i := uint64(0); i < uint64(ts.numOfTestDrummerNodes); i++ {
		addr := fmt.Sprintf("localhost:%d", port)
		ts.drummerAPIAddrs = append(ts.drummerAPIAddrs, addr)
		port++
	}
	for i := uint64(0); i < uint64(ts.numOfTestNodeHostNodes); i++ {
		addr := fmt.Sprintf("localhost:%d", port)
		ts.nodehostAPIAddrs = append(ts.nodehostAPIAddrs, addr)
		port++
	}
	return ts
}

func removeTestDir(fs config.IFS) {
	fs.RemoveAll(monkeyTestWorkingDir)
}

// TODO: need to save the test dir when running in the memfs mode
func saveTestDir() {
	newName := fmt.Sprintf("%s-%d", monkeyTestWorkingDir, rand.Uint64())
	plog.Infof("going to save the monkey test data dir to %s", newName)
	if err := os.Rename(monkeyTestWorkingDir, newName); err != nil {
		panic(err)
	}
}

func getTestConfig(ts *testSetup) (config.Config, config.NodeHostConfig) {
	lc := config.GetTinyMemLogDBConfig()
	lc.Shards = 1
	ec := config.GetDefaultEngineConfig()
	ec.ExecShards = 4
	ec.SnapshotShards = ts.snapshotWorkerCount
	ec.ApplyShards = ts.applyWorkerCount
	rc := config.Config{
		ElectionRTT:        20,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    100,
		CompactionOverhead: 100,
	}
	nhc := config.NodeHostConfig{
		WALDir:         "drummermt",
		NodeHostDir:    "drummermt",
		RTTMillisecond: 500,
		NotifyCommit:   true,
		Expert: config.ExpertConfig{
			LogDB:  lc,
			Engine: ec,
		},
	}
	return rc, nhc
}

type testNode struct {
	nodeType           nodeType
	next               uint64
	index              uint64
	cycle              uint64
	dir                string
	nh                 *dragonboat.NodeHost
	drummer            *Drummer
	server             *NodehostAPI
	drummerClient      *client.NodeHostClient
	drummerStopped     bool
	stopped            bool
	partitionTestNode  bool
	partitionStartTime map[uint64]struct{}
	partitionEndTime   map[uint64]struct{}
	stopper            *syncutil.Stopper
	fs                 config.IFS
	ts                 *testSetup
}

func (n *testNode) isDrummerNode() bool {
	return n.nodeType == nodeTypeDrummer
}

func (n *testNode) mustBeDrummer() {
	if n.nodeType != nodeTypeDrummer {
		panic("not drummer node")
	}
}

func (n *testNode) mustBeNodehost() {
	if n.nodeType != nodeTypeNodehost {
		panic("not nodehost node")
	}
}

func (n *testNode) removeNodeHostDir() {
	n.mustBeNodehost()
	idx := n.index + 1
	nn := fmt.Sprintf("nodehost-node-%d", idx)
	nd := n.fs.PathJoin(monkeyTestWorkingDir, nn)
	plog.Infof("monkey is going to delete nodehost dir at %s for testing", nd)
	if err := n.fs.RemoveAll(nd); err != nil {
		panic(err)
	}
}

func (n *testNode) isDrummerLeader() bool {
	n.mustBeDrummer()
	return n.drummer.isLeaderDrummerNode()
}

func (n *testNode) setupPartitionTests(seconds uint64) {
	st := n.ts.partitionMinStartSecond
	et := st
	plog.Infof("test node %d is set to run partition test", n.index+1)
	n.partitionTestNode = true
	for {
		partitionTime := rand.Uint64() % n.ts.partitionCycle
		if partitionTime < n.ts.partitionMinSecond {
			partitionTime = n.ts.partitionMinSecond
		}
		interval := rand.Uint64() % n.ts.partitionCycleInterval
		if interval < n.ts.partitionCycleMinInterval {
			interval = n.ts.partitionCycleMinInterval
		}
		st = et + interval
		et = st + partitionTime
		if st < seconds && et < seconds {
			plog.Infof("adding a partition cycle, st %d, et %d", st, et)
			n.partitionStartTime[st] = struct{}{}
			n.partitionEndTime[et] = struct{}{}
		} else {
			return
		}
	}
}

func (n *testNode) isPartitionTestNode() bool {
	return n.partitionTestNode
}

func (n *testNode) isRunning() bool {
	return n.nh != nil && !n.stopped
}

func (n *testNode) ignoreSync() {
	if memfs, ok := n.fs.(*dragonboat.MemFS); ok {
		plog.Infof("SetIgnoreSyncs called")
		n.nh.PartitionNode()
		memfs.SetIgnoreSyncs(true)
		time.Sleep(time.Second)
	}
}

func (n *testNode) allowSync() {
	if memfs, ok := n.fs.(*dragonboat.MemFS); ok {
		plog.Infof("ResetToSyncedState called")
		if n.nh != nil {
			n.nh.RestorePartitionedNode()
		}
		memfs.SetIgnoreSyncs(false)
		memfs.ResetToSyncedState()
	}
}

func (n *testNode) stop() {
	if n.stopped {
		panic("already stopped")
	}
	n.ignoreSync()
	n.stopped = true
	done := uint32(0)
	// see whether we can stop the node within reasonable timeframe
	go func() {
		count := uint64(0)
		for {
			time.Sleep(100 * time.Millisecond)
			if atomic.LoadUint32(&done) == 1 {
				break
			}
			count++
			if count == 10*n.ts.maxWaitForStopSecond {
				pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
				plog.Panicf("failed to stop nodehost %s, it is a %s, idx %d",
					n.nh.RaftAddress(), n.nodeType, n.index)
			}
		}
	}()
	addr := n.nh.RaftAddress()
	if n.nodeType == nodeTypeDrummer {
		plog.Infof("going to stop the drummer of %s", addr)
		if n.drummer != nil && !n.drummerStopped {
			n.drummer.Stop()
			n.drummerStopped = true
		}
		plog.Infof("the drummer part of %s stopped", addr)
	}
	plog.Infof("going to stop the nh of %s", addr)
	if n.server != nil {
		n.server.Stop()
	}
	if n.drummerClient != nil {
		n.drummerClient.Stop()
	}
	n.nh.Stop()
	plog.Infof("the nh part of %s stopped", addr)
	if n.stopper != nil {
		plog.Infof("monkey node has a stopper, %s", addr)
		n.stopper.Stop()
		plog.Infof("stopper on monkey %s stopped", addr)
	}
	atomic.StoreUint32(&done, 1)
}

func (n *testNode) start(ts *testSetup) {
	n.allowSync()
	n.cycle++
	if n.nodeType == nodeTypeDrummer {
		n.startDrummerNode(ts)
	} else if n.nodeType == nodeTypeNodehost {
		n.startNodehostNode(ts)
	} else {
		panic("unknown node type")
	}
}

func (n *testNode) compact() {
	if n.nodeType == nodeTypeNodehost {
		cid := rand.Uint64() % 128
		nh := n.nh
		for _, rn := range nh.Clusters() {
			if rn.ClusterID() == cid {
				plog.Infof("going to request a compaction for cluster %d", cid)
				sop, err := nh.RequestCompaction(cid, rn.NodeID())
				if err == dragonboat.ErrRejected {
					return
				}
				if err != nil {
					plog.Panicf("failed to request compaction %v", err)
				}
				<-sop.CompletedC()
				plog.Infof("cluster %d compaction completed", cid)
			}
		}
	}
}

func (n *testNode) getClustersAndTick() (*multiCluster, uint64, error) {
	n.mustBeDrummer()
	sc, err := n.drummer.getSchedulerContext()
	if err != nil {
		return nil, 0, err
	}
	return sc.ClusterImage, sc.Tick, nil
}

func (n *testNode) getClusters() (*multiCluster, error) {
	n.mustBeDrummer()
	sc, err := n.drummer.getSchedulerContext()
	if err != nil {
		return nil, err
	}
	return sc.ClusterImage, nil
}

func (n *testNode) setNodeNext(low uint64, high uint64) {
	if high <= low {
		panic("high <= low")
	}
	v := low + rand.Uint64()%(high-low)
	plog.Infof("next event for %s %d scheduled in %d second",
		n.nodeType, n.index+1, v)
	n.next = n.next + v
}

func (n *testNode) startDrummerNode(ts *testSetup) {
	n.mustBeDrummer()
	if !n.stopped {
		panic("already running")
	}
	rc, nhc := getTestConfig(ts)
	config := config.NodeHostConfig{}
	config = nhc
	config.NodeHostDir = n.fs.PathJoin(n.dir, nhc.NodeHostDir)
	config.WALDir = n.fs.PathJoin(n.dir, nhc.WALDir)
	config.RaftAddress = ts.drummerAddrs[n.index]
	config.Expert.FS = n.fs
	nh, err := dragonboat.NewNodeHost(config)
	if err != nil {
		panic(err)
	}
	n.nh = nh
	peers := make(map[uint64]string)
	for idx, v := range ts.drummerAddrs {
		peers[uint64(idx+1)] = v
	}
	rc.NodeID = uint64(n.index + 1)
	rc.ClusterID = defaultClusterID
	if err := nh.StartCluster(peers, false, NewDB, rc); err != nil {
		panic(err)
	}
	addr := ts.drummerAPIAddrs[n.index]
	drummerServer := NewDrummer(nh, addr)
	drummerServer.Start()
	n.drummer = drummerServer
	n.drummerStopped = false
	n.stopped = false
	n.stopper = syncutil.NewStopper()
}

func (n *testNode) startNodehostNode(ts *testSetup) {
	if n.nodeType != nodeTypeNodehost {
		panic("trying to start a drummer on a non-drummer node")
	}
	if !n.stopped {
		panic("already running")
	}
	_, nhc := getTestConfig(ts)
	config := config.NodeHostConfig{}
	config = nhc
	config.NodeHostDir = n.fs.PathJoin(n.dir, nhc.NodeHostDir)
	config.WALDir = n.fs.PathJoin(n.dir, nhc.WALDir)
	config.RaftAddress = ts.nodehostAddrs[n.index]
	config.Expert.FS = n.fs
	if n.index == uint64(len(ts.nodehostAddrs))-1 {
		plog.Infof("using a much higher RTTMillisecond for %s", config.RaftAddress)
		config.RTTMillisecond = config.RTTMillisecond * 3
	}
	addr := ts.nodehostAPIAddrs[n.index]
	nh, err := dragonboat.NewNodeHost(config)
	if err != nil {
		panic(err)
	}
	n.nh = nh
	n.drummerClient = client.NewNodeHostClient(nh, ts.drummerAPIAddrs, addr)
	n.server = NewNodehostAPI(addr, nh)
	n.stopped = false
}

type testEnv struct {
	ts               *testSetup
	nodehosts        []*testNode
	drummers         []*testNode
	low              uint64
	high             uint64
	second           uint64
	deleteDataTested bool
	stopper          *syncutil.Stopper
	completedIO      uint64
}

func createTestNodes(ts *testSetup) *testEnv {
	te := &testEnv{
		drummers:  make([]*testNode, len(ts.drummerAddrs)),
		nodehosts: make([]*testNode, len(ts.nodehostAddrs)),
		ts:        ts,
		low:       ts.nodeUpTimeLowSecond,
		high:      ts.nodeUpTimeHighSecond,
		stopper:   syncutil.NewStopper(),
	}
	for i := uint64(0); i < uint64(len(ts.drummerAddrs)); i++ {
		fs := dragonboat.GetTestFS()
		if _, ok := fs.(*dragonboat.MemFS); ok {
			plog.Infof("drummer %d using memfs", i)
		}
		te.drummers[i] = &testNode{
			index:              i,
			stopped:            true,
			dir:                fs.PathJoin(monkeyTestWorkingDir, ts.drummerDirs[i]),
			nodeType:           nodeTypeDrummer,
			partitionStartTime: make(map[uint64]struct{}),
			partitionEndTime:   make(map[uint64]struct{}),
			fs:                 fs,
			ts:                 ts,
		}
		removeTestDir(fs)
	}
	for i := uint64(0); i < uint64(len(ts.nodehostAddrs)); i++ {
		fs := dragonboat.GetTestFS()
		if _, ok := fs.(*dragonboat.MemFS); ok {
			plog.Infof("nodehost %d using memfs", i)
		}
		te.nodehosts[i] = &testNode{
			index:              i,
			stopped:            true,
			dir:                fs.PathJoin(monkeyTestWorkingDir, ts.nodehostDirs[i]),
			nodeType:           nodeTypeNodehost,
			partitionStartTime: make(map[uint64]struct{}),
			partitionEndTime:   make(map[uint64]struct{}),
			fs:                 fs,
			ts:                 ts,
		}
		removeTestDir(fs)
	}
	return te
}

func (te *testEnv) ensureNodeHostNotPartitioned(t *testing.T) {
	for _, n := range te.nodehosts {
		n.mustBeNodehost()
		if n.nh.IsPartitioned() {
			t.Fatalf("nodehost is still in partitioned mode")
		}
	}
}

func (te *testEnv) checkRateLimiterState(t *testing.T, last bool) bool {
	if rateLimiterDisabledInConfig() {
		return true
	}
	for _, n := range te.nodehosts {
		n.mustBeNodehost()
		nh := n.nh
		for _, rn := range nh.Clusters() {
			rl := rn.GetRateLimiter()
			clusterID := rn.ClusterID()
			nodeID := rn.NodeID()
			if rl.Get() != rn.GetInMemLogSize() {
				if last {
					t.Fatalf("%s, rl mem log size %d, in mem log size %d",
						dn(clusterID, nodeID), rl.Get(), rn.GetInMemLogSize())
				}
				return false
			}
		}
	}
	return true
}

func (te *testEnv) checkNodeHostsSynced(t *testing.T, last bool) bool {
	return te.checkNodesSynced(t, te.nodehosts, last)
}

func (te *testEnv) checkDrummersSynced(t *testing.T, last bool) bool {
	return te.checkNodesSynced(t, te.drummers, last)
}

func (te *testEnv) checkNodesSynced(t *testing.T, nodes []*testNode, last bool) bool {
	appliedMap := make(map[uint64]uint64)
	notSynced := make(map[uint64]struct{})
	for _, n := range nodes {
		nh := n.nh
		for _, rn := range nh.Clusters() {
			clusterID := rn.ClusterID()
			lastApplied := rn.GetLastApplied()
			existingLastApplied, ok := appliedMap[clusterID]
			if !ok {
				appliedMap[clusterID] = lastApplied
			} else {
				if existingLastApplied != lastApplied {
					notSynced[clusterID] = struct{}{}
				}
			}
		}
	}
	if len(notSynced) > 0 {
		if last {
			logCluster(nodes, notSynced)
			t.Fatalf("failed to sync all nodes")
		}
		return false
	}
	return true
}

func (te *testEnv) checkLogDBSynced(t *testing.T, last bool) bool {
	if snapshotDisabledInConfig() {
		return te.logDBSynced(t, te.nodehosts, last)
	}
	return true
}

func (te *testEnv) logDBSynced(t *testing.T, nodes []*testNode, last bool) bool {
	hashMap := make(map[uint64]uint64)
	notSynced := make(map[uint64]struct{})
	for _, n := range nodes {
		nh := n.nh
		for _, rn := range nh.Clusters() {
			nodeID := rn.NodeID()
			clusterID := rn.ClusterID()
			lastApplied := rn.GetLastApplied()
			logdb := nh.GetLogDB()
			entries, _, err := logdb.IterateEntries(nil,
				0, clusterID, nodeID, 1, lastApplied+1, math.MaxUint64)
			if err != nil {
				t.Fatalf("failed to get entries %v", err)
			}
			hash := getEntryListHash(entries)
			plog.Infof("%s logdb entry hash %d, last applied %d, ent sz %d",
				dn(clusterID, nodeID),
				hash, lastApplied, len(entries))
			printEntries(clusterID, nodeID, entries)
			existingHash, ok := hashMap[clusterID]
			if !ok {
				hashMap[clusterID] = hash
			} else {
				if existingHash != hash {
					notSynced[clusterID] = struct{}{}
				}
			}
		}
	}
	if len(notSynced) > 0 {
		logCluster(te.nodehosts, notSynced)
		if last {
			t.Fatalf("%d clusters failed to have logDB synced, %v",
				len(notSynced), notSynced)
		}
		return false
	}
	return true
}

func (te *testEnv) checkNodeHostSM(t *testing.T, last bool) bool {
	return te.checkStateMachine(t, te.nodehosts, last)
}

func (te *testEnv) checkDrummerSM(t *testing.T, last bool) bool {
	return te.checkStateMachine(t, te.drummers, last)
}

func (te *testEnv) checkStateMachine(t *testing.T, nodes []*testNode, last bool) bool {
	hashMap := make(map[uint64]uint64)
	sessionHashMap := make(map[uint64]uint64)
	membershipMap := make(map[uint64]uint64)
	inconsistent := make(map[uint64]struct{})
	for _, n := range nodes {
		nh := n.nh
		for _, rn := range nh.Clusters() {
			clusterID := rn.ClusterID()
			hash := rn.GetStateMachineHash()
			sessionHash := rn.GetSessionHash()
			membershipHash := rn.GetMembershipHash()
			// check hash
			existingHash, ok := hashMap[clusterID]
			if !ok {
				hashMap[clusterID] = hash
			} else {
				if existingHash != hash {
					inconsistent[clusterID] = struct{}{}
					if last {
						t.Errorf("hash mismatch, cluster id %d, existing %d, new %d",
							clusterID, existingHash, hash)
					}
				}
			}
			// check session hash
			existingHash, ok = sessionHashMap[clusterID]
			if !ok {
				sessionHashMap[clusterID] = sessionHash
			} else {
				if existingHash != sessionHash {
					inconsistent[clusterID] = struct{}{}
					if last {
						t.Errorf("session hash mismatch, cluster id %d, existing %d, new %d",
							clusterID, existingHash, sessionHash)
					}
				}
			}
			// check membership
			existingHash, ok = membershipMap[clusterID]
			if !ok {
				membershipMap[clusterID] = membershipHash
			} else {
				if existingHash != membershipHash {
					inconsistent[clusterID] = struct{}{}
					if last {
						t.Errorf("membership hash mismatch, cluster id %d, %d vs %d",
							clusterID, existingHash, membershipHash)
					}
				}
			}
		}
	}
	if len(inconsistent) > 0 && last {
		logCluster(nodes, inconsistent)
		t.Fatalf("inconsistent sm state found")
	}
	return len(inconsistent) == 0
}

func (te *testEnv) startNodeHostNodes(startWorkers bool) {
	te.startNodes(te.nodehosts)
	if startWorkers {
		for _, n := range te.nodehosts {
			te.startResponseChecker(n.nh)
		}
	}
}

func (te *testEnv) startDrummerNodes() {
	te.startNodes(te.drummers)
}

func (te *testEnv) startNodes(nodes []*testNode) {
	for _, n := range nodes {
		if !n.isRunning() {
			n.start(te.ts)
		}
	}
}

func (te *testEnv) stopNodeHostNodes() {
	te.stopNodes(te.nodehosts)
}

func (te *testEnv) stopDrummerNodes() {
	te.stopNodes(te.drummers)
}

func (te *testEnv) stopNodes(nodes []*testNode) {
	for _, n := range nodes {
		if n.isRunning() {
			n.stop()
		}
	}
}

func (te *testEnv) checkClustersLaunched(t *testing.T, last bool) bool {
	clusters := make(map[uint64]uint64)
	clustersReady := func(cs map[uint64]uint64) bool {
		if uint64(len(cs)) != te.ts.numOfClusters {
			return false
		}
		for _, count := range cs {
			if count != 3 {
				return false
			}
		}
		return true
	}
	for _, tn := range te.nodehosts {
		nh := tn.nh
		for _, node := range nh.Clusters() {
			if count, ok := clusters[node.ClusterID()]; ok {
				clusters[node.ClusterID()] = count + 1
			} else {
				clusters[node.ClusterID()] = 1
			}
		}
	}
	ready := clustersReady(clusters)
	if last && !ready {
		t.Fatalf("not all clusters are launched")
	}
	return ready
}

func (te *testEnv) waitForNodeHosts() {
	waitForStableNodes(te.nodehosts, te.ts.waitForStableSecond)
}

func (te *testEnv) waitForDrummers() {
	waitForStableNodes(te.drummers, te.ts.waitForStableSecond)
}

func waitForStableNodes(nodes []*testNode, seconds uint64) bool {
	waitInBetweenSecond := time.Duration(3)
	time.Sleep(waitInBetweenSecond * time.Second)
	tryWait := func(nodes []*testNode, seconds uint64) bool {
		waitMilliseconds := seconds * 1000
		totalWait := uint64(0)
		var nodeReady bool
		var leaderReady bool
		for !nodeReady || !leaderReady {
			nodeReady = true
			leaderReady = true
			leaderMap := make(map[uint64]struct{})
			clusterSet := make(map[uint64]struct{})
			time.Sleep(100 * time.Millisecond)
			totalWait += 100
			if totalWait >= waitMilliseconds {
				return false
			}
			for _, node := range nodes {
				if node == nil || node.nh == nil {
					continue
				}
				nh := node.nh
				clusters := nh.Clusters()
				for _, rn := range clusters {
					clusterSet[rn.ClusterID()] = struct{}{}
					isLeader := rn.IsLeader()
					isFollower := rn.IsFollower()
					if !isLeader && !isFollower {
						nodeReady = false
					}

					if isLeader {
						leaderMap[rn.ClusterID()] = struct{}{}
					}
				}
			}
			if len(leaderMap) != len(clusterSet) {
				leaderReady = false
			}
		}
		return true
	}
	for {
		if done := tryWait(nodes, seconds); !done {
			return false
		}
		time.Sleep(waitInBetweenSecond * time.Second)
		if done := tryWait(nodes, seconds); done {
			return true
		}
		time.Sleep(waitInBetweenSecond * time.Second)
	}
}

func (te *testEnv) brutalMonkeyPlay() {
	for _, nodes := range [][]*testNode{te.nodehosts, te.drummers} {
		for _, n := range nodes {
			if !n.isPartitionTestNode() && n.isRunning() {
				n.stop()
				plog.Infof("monkey brutally stopped %s %d", n.nodeType, n.index+1)
				n.setNodeNext(te.low, te.high)
			}
		}
	}
}

func (te *testEnv) monkeyPlay() {
	for _, nodes := range [][]*testNode{te.nodehosts, te.drummers} {
		for _, n := range nodes {
			if !n.isPartitionTestNode() {
				// crash mode
				if n.next == 0 {
					n.setNodeNext(te.low, te.high)
					continue
				} else if n.next > te.second {
					continue
				}
				if rand.Uint64()%100 == 0 && n.isRunning() {
					n.compact()
				}
				if n.isRunning() {
					plog.Infof("monkey will stop %s %d", n.nodeType, n.index+1)
					n.stop()
					plog.Infof("monkey stopped %s %d", n.nodeType, n.index+1)
					if rand.Uint64()%5 == 0 &&
						!n.isDrummerNode() && !te.deleteDataTested && te.second < 800 {
						plog.Infof("monkey will delete all on %s %d", n.nodeType, n.index+1)
						n.removeNodeHostDir()
						te.deleteDataTested = true
					}
				} else {
					plog.Infof("monkey will start %s %d", n.nodeType, n.index+1)
					n.start(te.ts)
					te.startResponseChecker(n.nh)
					plog.Infof("monkey restarted %s %d", n.nodeType, n.index+1)
				}
				n.setNodeNext(te.low, te.high)
			} else {
				// partition mode
				if _, ps := n.partitionStartTime[te.second]; ps {
					plog.Infof("monkey partitioning the node %d", n.index+1)
					n.nh.PartitionNode()
				}
				if _, pe := n.partitionEndTime[te.second]; pe {
					plog.Infof("monkey restoring node %d from partition mode", n.index+1)
					n.nh.RestorePartitionedNode()
				}
			}
		}
	}
}

func (te *testEnv) stopDrummerActivity() {
	for _, n := range te.nodehosts {
		if n.drummerClient != nil {
			n.drummerClient.StopNodeHostInfoReporter()
		}
	}
	for _, n := range te.drummers {
		n.stopper.Stop()
		n.stopper = nil
		n.drummer.Stop()
		n.drummer.ctx, n.drummer.cancel = context.WithCancel(context.Background())
		n.drummerStopped = true
	}
}

func getDrummerClient(drummerAddressList []string) (pb.DrummerClient, *client.Connection) {
	pool := client.NewDrummerConnectionPool()
	for _, server := range drummerAddressList {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
		conn, err := pool.GetInsecureConnection(ctx, server)
		cancel()
		if err == nil {
			return pb.NewDrummerClient(conn.ClientConn()), conn
		}
	}

	return nil, nil
}

func submitClusters(count uint64,
	name string, dclient pb.DrummerClient) error {
	plog.Infof("going to send cluster info to drummer")
	for i := uint64(0); i < count; i++ {
		clusterID := i + 1
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
		if err := SubmitCreateDrummerChange(ctx,
			dclient, clusterID, []uint64{defaultNodeID1, defaultNodeID2, defaultNodeID3}, name); err != nil {
			plog.Errorf("failed to submit drummer change, cluster %d, %v",
				clusterID, err)
			cancel()
			return err
		}
		cancel()
	}
	regions := pb.Regions{
		Region: []string{client.DefaultRegion},
		Count:  []uint64{3},
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestTimeout)
	defer cancel()
	plog.Infof("going to set region")
	if err := SubmitRegions(ctx, dclient, regions); err != nil {
		plog.Errorf("failed to submit region info")
		return err
	}
	plog.Infof("going to set the bootstrapped flag")
	if err := SubmitBootstrappped(ctx, dclient); err != nil {
		plog.Errorf("failed to set bootstrapped flag")
		return err
	}

	return nil
}

func (te *testEnv) submitJobs(name string) bool {
	for i := 0; i < 5; i++ {
		dc, connection := getDrummerClient(te.ts.drummerAPIAddrs)
		if dc == nil {
			continue
		}
		defer connection.Close()
		if err := submitClusters(te.ts.numOfClusters, name, dc); err == nil {
			return true
		}
		time.Sleep(time.Duration(NodeHostInfoReportSecond) * time.Second)
	}
	return false
}

func (te *testEnv) checkClustersAreAccessible(t *testing.T) {
	if te.ts.slowvm {
		plog.Infof("running on slow vm, availability check skipped")
		return
	}
	synced := make(map[uint64]struct{})
	timeout := defaultTestTimeout
	count := te.ts.numOfClusters
	for iteration := 0; iteration < 100; iteration++ {
		for clusterID := uint64(1); clusterID <= count; clusterID++ {
			if _, ok := synced[clusterID]; ok {
				continue
			}
			plog.Infof("checking cluster availability for %d", clusterID)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			if te.makeMonkeyRequests(ctx, clusterID, false) {
				synced[clusterID] = struct{}{}
			}
			cancel()
		}
		if uint64(len(synced)) == count {
			return
		}
	}
	t.Fatalf("%d clusters are not accessible", count-uint64(len(synced)))
}

func (te *testEnv) getRequestAddress(ctx context.Context,
	clusterID uint64) (string, string, bool) {
	p := client.NewDrummerConnectionPool()
	defer p.Close()
	var conn *client.Connection
	var err error
	for idx := 0; idx < len(te.ts.drummerAPIAddrs); idx++ {
		cctx, cancel := context.WithTimeout(ctx, defaultTestTimeout)
		addr := te.ts.drummerAPIAddrs[idx]
		conn, err = p.GetInsecureConnection(cctx, addr)
		cancel()
		if err != nil {
			plog.Infof("failed to get drummer connection, %s, %v", addr, err)
			continue
		}
	}
	if conn == nil {
		plog.Infof("failed to get any connection")
		return "", "", false
	}
	client := pb.NewDrummerClient(conn.ClientConn())
	req := &pb.ClusterStateRequest{ClusterIdList: []uint64{clusterID}}
	resp, err := client.GetClusterStates(ctx, req)
	if err != nil {
		plog.Warningf("failed to get cluster info %v", err)
		return "", "", false
	}
	if len(resp.Collection) != 1 {
		plog.Warningf("collection size is not 1")
		return "", "", false
	}
	ci := resp.Collection[0]
	writeNodeAddress := ""
	readNodeAddress := ""
	readNodeIdx := rand.Int() % len(ci.RPCAddresses)
	writeNodeIdx := rand.Int() % len(ci.RPCAddresses)
	nodeIDList := make([]uint64, 0)
	for nodeID := range ci.RPCAddresses {
		nodeIDList = append(nodeIDList, nodeID)
	}
	for nodeID, addr := range ci.RPCAddresses {
		if nodeID == nodeIDList[writeNodeIdx] {
			writeNodeAddress = addr
		}
		if nodeID == nodeIDList[readNodeIdx] {
			readNodeAddress = addr
		}
	}
	if len(readNodeAddress) == 0 || len(writeNodeAddress) == 0 {
		plog.Warningf("failed to set read/write addresses")
		return "", "", false
	}
	return writeNodeAddress, readNodeAddress, true
}

func getMonkeyTestClients(ctx context.Context,
	p *client.Pool, writeAddress string,
	readAddress string) (mr.NodehostAPIClient, mr.NodehostAPIClient) {
	writeConn, err := p.GetInsecureConnection(ctx, writeAddress)
	if err != nil {
		plog.Warningf("failed to connect to the write nodehost, %v", err)
		return nil, nil
	}
	writeClient := mr.NewNodehostAPIClient(writeConn.ClientConn())
	readConn, err := p.GetInsecureConnection(ctx, readAddress)
	if err != nil {
		plog.Warningf("failed to connect to the read nodehost, %v", err)
		return nil, nil
	}
	readClient := mr.NewNodehostAPIClient(readConn.ClientConn())
	return readClient, writeClient
}

func makeWriteRequest(ctx context.Context,
	client mr.NodehostAPIClient, clusterID uint64, kv *kv.KV) bool {
	data, err := kv.MarshalBinary()
	if err != nil {
		panic(err)
	}
	// get client session
	req := &mr.SessionRequest{ClusterId: clusterID}
	cs, err := client.GetSession(ctx, req)
	if err != nil {
		plog.Warningf("failed to get client session for cluster %d, %v",
			clusterID, err)
		return false
	}
	defer client.CloseSession(ctx, cs)
	raftProposal := &mr.RaftProposal{
		Session: *cs,
		Data:    data,
	}
	resp, err := client.Propose(ctx, raftProposal)
	if err == nil {
		if resp.Result != uint64(len(data)) {
			plog.Panicf("result %d, want %d", resp.Result, uint64(len(data)))
		}
		if !cs.IsNoOPSession() {
			cs.ProposalCompleted()
		}
	} else {
		plog.Warningf("failed to make proposal %v", err)
		return false
	}
	return true
}

func makeReadRequest(ctx context.Context,
	client mr.NodehostAPIClient, clusterID uint64, kv *kv.KV) bool {
	ri := &mr.RaftReadIndex{
		ClusterId: clusterID,
		Data:      []byte(kv.Key),
	}
	resp, err := client.Read(ctx, ri)
	if err != nil {
		plog.Warningf("failed to read, %v", err)
		return false
	} else {
		if string(resp.Data) != kv.Val {
			plog.Panicf("inconsistent state, got %s, want %s",
				string(resp.Data), kv.Val)
		}
	}
	return true
}

func (te *testEnv) makeMonkeyRequests(ctx context.Context,
	clusterID uint64, repeated bool) bool {
	writeAddr, readAddr, ok := te.getRequestAddress(ctx, clusterID)
	if !ok {
		plog.Infof("failed to get read write address")
		return false
	}
	pool := client.NewConnectionPool()
	defer pool.Close()
	readClient, writeClient := getMonkeyTestClients(ctx, pool, writeAddr, readAddr)
	if writeClient == nil || readClient == nil {
		plog.Warningf("failed to get read write client")
		return false
	}
	repeat := 1
	if repeated {
		repeat = rand.Int()%3 + 1
	}
	for i := 0; i < repeat; i++ {
		key := fmt.Sprintf("key-%d", rand.Uint64())
		val := random.String(rand.Int()%16 + 8)
		kv := &kv.KV{
			Key: key,
			Val: val,
		}
		cctx, cancel := context.WithTimeout(ctx, 2*defaultTestTimeout)
		if makeWriteRequest(cctx, writeClient, clusterID, kv) {
			if !makeReadRequest(cctx, readClient, clusterID, kv) {
				cancel()
				return false
			} else {
				atomic.AddUint64(&te.completedIO, 1)
			}
		} else {
			cancel()
			return false
		}
		cancel()
	}
	return true
}

func (te *testEnv) stopWorkers() {
	te.stopper.Stop()
}

func (te *testEnv) checkProposalResponse(nh *dragonboat.NodeHost) bool {
	if nh.Stopped() {
		return false
	}
	clusterID := rand.Uint64()%te.ts.numOfClusters + 1
	nodeID := []uint64{defaultNodeID1, defaultNodeID2, defaultNodeID3}[rand.Uint64()%3]
	if err := nh.RequestLeaderTransfer(clusterID, nodeID); err != nil {
		plog.Errorf("leader transfer request failed, %v", err)
	}
	session := nh.GetNoOPSession(clusterID)
	kv := &kv.KV{
		Key: fmt.Sprintf("proposal-response-check-key-%d", rand.Uint64()),
		Val: fmt.Sprintf("proposal-response-check-val-%d", rand.Uint64()),
	}
	data, err := kv.MarshalBinary()
	if err != nil {
		panic(err)
	}
	plog.Infof("making a test proposal on %s, cluster %d, %d bytes",
		nh.RaftAddress(), clusterID, len(data))
	rs, err := nh.Propose(session, data, 10*time.Second)
	if err == dragonboat.ErrClosed || err == dragonboat.ErrClusterClosed {
		return false
	}
	if err != nil {
		plog.Errorf("propose failed %v", err)
		return true
	}
	wait := 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-te.stopper.ShouldStop():
			return false
		case <-ticker.C:
			wait++
			if wait%10 == 0 {
				plog.Infof("waited %d seconds, cluster %d", wait, clusterID)
			}
			if wait == 18 {
				select {
				case <-rs.AppliedC():
					return true
				default:
				}
				plog.Panicf("failed to get response, cluster %d, nh %s",
					clusterID, nh.RaftAddress())
			}
		case <-rs.AppliedC():
			return true
		}
	}
	return true
}

func (te *testEnv) checkSnapshotOp(nh *dragonboat.NodeHost) bool {
	if nh.Stopped() || snapshotDisabledInConfig() {
		return false
	}
	clusterID := rand.Uint64()%te.ts.numOfClusters + 1
	clusterID2 := rand.Uint64()%te.ts.numOfClusters + 1
	nodeID := []uint64{defaultNodeID1, defaultNodeID2, defaultNodeID3}[rand.Uint64()%3]
	rs, err := nh.RequestSnapshot(clusterID, dragonboat.DefaultSnapshotOption, 2*time.Second)
	if err != nil {
		return true
	}
	rs2, err := nh.RequestCompaction(clusterID2, nodeID)
	if err != nil {
		return true
	}
	select {
	case <-rs.CompletedC:
		return true
	case <-te.stopper.ShouldStop():
		return false
	case <-rs2.CompletedC():
		return true
	}
}

func (te *testEnv) startResponseChecker(nh *dragonboat.NodeHost) {
	te.stopper.RunWorker(func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		slowTicker := time.NewTicker(30 * time.Second)
		defer slowTicker.Stop()
		for {
			select {
			case <-te.stopper.ShouldStop():
				return
			case <-ticker.C:
				if !te.checkProposalResponse(nh) {
					return
				}
			case <-slowTicker.C:
				if !te.checkSnapshotOp(nh) {
					return
				}
			}
		}
	})
}

func (te *testEnv) startFastWorker() {
	te.stopper.RunWorker(func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-te.stopper.ShouldStop():
				return
			default:
			}
			select {
			case <-ticker.C:
				for i := 0; i < 100; i++ {
					if cont := func() bool {
						timeout := defaultTestTimeout
						clusterID := client.HardWorkerTestClusterID
						ctx, cancel := context.WithTimeout(context.Background(), timeout)
						defer cancel()
						if !te.makeMonkeyRequests(ctx, clusterID, false) {
							return false
						}
						return true
					}(); !cont {
						break
					}
				}
			}
		}
	})
}

func (te *testEnv) startRequestWorkers() {
	for i := uint64(0); i < te.ts.testClientWorkerCount; i++ {
		te.stopper.RunWorker(func() {
			tick := 0
			lastDone := 0
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-te.stopper.ShouldStop():
					return
				default:
				}
				select {
				case <-ticker.C:
					tick++
					if tick-lastDone > 5 {
						timeout := 3 * defaultTestTimeout
						clusterID := getRandomClusterID(te.ts.numOfClusters)
						ctx, cancel := context.WithTimeout(context.Background(), timeout)
						if te.makeMonkeyRequests(ctx, clusterID, true) {
							lastDone = tick
						}
						cancel()
					}
				}
			}
		})
	}
}

func (te *testEnv) randomDropPacket(enabled bool) {
	threshold := uint64(0)
	if enabled {
		threshold = 1
	}
	// both funcs below return a shouldSend boolean value
	hook := func(batch raftpb.MessageBatch) (raftpb.MessageBatch, bool) {
		pd := random.NewProbability(1000 * threshold)
		if pd.Hit() {
			return raftpb.MessageBatch{}, false
		}
		pdr := random.NewProbability(5000 * threshold)
		if pdr.Hit() && len(batch.Requests) > 1 {
			dropIdx := random.LockGuardedRand.Uint64() % uint64(len(batch.Requests))
			reqs := make([]raftpb.Message, 0)
			for idx, req := range batch.Requests {
				if uint64(idx) != dropIdx {
					reqs = append(reqs, req)
				}
			}
			if len(reqs) != len(batch.Requests)-1 {
				panic("message not internally dropped")
			}
			batch.Requests = reqs
		}
		return batch, true
	}
	snapshotHook := func(c raftpb.Chunk) (raftpb.Chunk, bool) {
		sd := random.NewProbability(1000 * threshold)
		if sd.Hit() {
			return raftpb.Chunk{}, false
		}
		return c, true
	}
	for _, n := range te.nodehosts {
		n.nh.SetTransportDropBatchHook(hook)
		n.nh.SetPreStreamChunkSendHook(snapshotHook)
	}
	for _, n := range te.drummers {
		n.nh.SetTransportDropBatchHook(hook)
		n.nh.SetPreStreamChunkSendHook(snapshotHook)
	}
}

func (te *testEnv) checkDrummerLeaderReady(t *testing.T, last bool) bool {
	return te.isDrummerReady(t, true, last)
}

func (te *testEnv) checkDrummerIsReady(t *testing.T, last bool) bool {
	return te.isDrummerReady(t, false, last)
}

func (te *testEnv) isDrummerReady(t *testing.T,
	checkLeaderOnly bool, last bool) bool {
	leaderChecked := false
	for _, n := range te.drummers {
		if !n.isRunning() {
			panic("drummer node not running?")
		}
		if !n.isDrummerLeader() {
			continue
		}
		leaderChecked = true
		if !checkLeaderOnly {
			mc, err := n.getClusters()
			if err != nil {
				if last {
					t.Fatalf("failed to get multiCluster %v", err)
				}
				return false
			}
			plog.Infof("num of clusters known to drummer %d", mc.size())
			if uint64(mc.size()) != te.ts.numOfClusters {
				if last {
					t.Fatalf("cluster count %d, want %d", mc.size(), te.ts.numOfClusters)
				}
				return false
			}
		}
	}
	if last && !leaderChecked {
		t.Fatalf("drummer leader is not ready")
	}
	return leaderChecked
}

func (te *testEnv) checkHeapSize(t *testing.T) {
	if te.second > te.ts.testIdleTime && te.second%10 == 0 {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		if memStats.HeapAlloc > te.ts.maxAllowedHeapSize {
			saveHeapProfile("drummer_mem_limit.pprof")
			t.Fatalf("heap size reached max allowed limit")
		}
	}
}

func (te *testEnv) monkeyTest(t *testing.T) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	brutalMonkeyTime := rand.Uint64()%200 + te.ts.monkeyTestSecondToRun/2
	for i := uint64(0); i < te.ts.monkeyTestSecondToRun; i++ {
		te.checkHeapSize(t)
		select {
		case <-ticker.C:
			te.second++
			if te.second < te.ts.testIdleTime {
				continue
			}
			if te.second == brutalMonkeyTime {
				plog.Infof("brutal monkey play is going to start, time %d", te.second)
				te.brutalMonkeyPlay()
			}
			te.monkeyPlay()
		}
	}
}

func (te *testEnv) checkClusterState(t *testing.T, last bool) bool {
	node := te.drummers[rand.Uint64()%uint64(len(te.drummers))]
	mc, tick, err := node.getClustersAndTick()
	if err != nil {
		if last {
			t.Fatalf("failed to get multiCluster, %v", err)
		}
		return false
	}
	if uint64(mc.size()) != te.ts.numOfClusters {
		if last {
			t.Fatalf("cluster count %d, want %d", mc.size(), te.ts.numOfClusters)
		}
		return false
	}
	toFix := make(map[uint64]struct{})
	rc := mc.getClusterForRepair(tick)
	for _, cr := range rc {
		toFix[cr.clusterID] = struct{}{}
	}
	uc := mc.getUnavailableClusters(tick)
	for _, cr := range uc {
		toFix[cr.ClusterID] = struct{}{}
	}
	if last {
		if len(rc) > 0 {
			t.Errorf("to be repaired cluster %d, want 0", len(rc))
			logClusterToRepair(rc, tick)
		}
		if len(uc) > 0 {
			t.Errorf("unavailable cluster %d, want 0", len(uc))
			logUnavailableCluster(uc, tick)
		}
		if len(toFix) > 0 {
			t.Errorf("to fix cluster %d, want 0", len(toFix))
			logCluster(te.nodehosts, toFix)
		}
	}
	return len(rc) == 0 && len(uc) == 0
}

type drummerCheck func(*testing.T, bool) bool

func check(t *testing.T, dc drummerCheck, iteration uint64) {
	for i := uint64(0); i < iteration; i++ {
		last := i == (iteration - 1)
		if dc(t, last) {
			return
		}
		if !last {
			time.Sleep(time.Duration(loopIntervalSecond) * time.Second)
		}
	}
}

func drummerMonkeyTesting(t *testing.T, to *testOption, name string) {
	defer func() {
		if r := recover(); r != nil || t.Failed() {
			plog.Infof("test failed, going to save the monkey test dir")
			saveTestDir()
			panic("core dump is required")
		} else {
			removeTestDir(dragonboat.GetTestFS())
		}
	}()
	plog.Infof("test pid %d", os.Getpid())
	plog.Infof("snapshot disabled in monkey test %t, less snapshot %t",
		snapshotDisabledInConfig(), lessSnapshotTest())
	ts := newTestSetup(to)
	te := createTestNodes(ts)
	te.startDrummerNodes()
	te.startNodeHostNodes(true)
	defer func() {
		plog.Infof("cleanup called, going to stop drummer nodes")
		te.stopDrummerNodes()
		plog.Infof("cleanup called, drummer nodes stopped")
	}()
	defer func() {
		plog.Infof("cleanup called, going to stop nodehost nodes")
		te.stopNodeHostNodes()
		plog.Infof("cleanup called, nodehost nodes stopped")
	}()
	plog.Infof("waiting for drummer nodes to stablize")
	te.waitForDrummers()
	plog.Infof("waiting for nodehost nodes to stablize")
	te.waitForNodeHosts()
	plog.Infof("all nodes are ready")
	// the first nodehost will use partition mode, all other nodes will use crash
	// mode for testing
	partitionTestDuration := te.ts.monkeyTestSecondToRun - 100
	te.nodehosts[0].setupPartitionTests(partitionTestDuration)
	// with 50% chance, we let more than one nodehostNode to do partition test
	if rand.Uint64()%2 == 0 {
		te.nodehosts[1].setupPartitionTests(partitionTestDuration)
	}
	check(t, te.checkDrummerLeaderReady, 10)
	plog.Infof("going to submit jobs")
	if !te.submitJobs(name) {
		t.Fatalf("failed to submit the test job")
	}
	plog.Infof("jobs submitted, waiting for clusters to be launched")
	check(t, te.checkClustersLaunched, 30)
	plog.Infof("all clusters launched, wait for drummer to be ready")
	check(t, te.checkDrummerIsReady, 10)
	plog.Infof("drummer is ready")
	te.randomDropPacket(true)
	te.startRequestWorkers()
	if lessSnapshotTest() {
		plog.Infof("going to start fast worker")
		disableClusterRandomDelay(client.HardWorkerTestClusterID)
		te.startFastWorker()
	}
	checker := lcm.NewCoordinator(context.Background(),
		te.ts.LCMWorkerCount, 1, te.ts.drummerAPIAddrs)
	checker.Start()
	plog.Infof("going to start the monkey test")
	te.monkeyTest(t)
	plog.Infof("going to stop the test clients")
	// stop all client workers
	te.stopWorkers()
	plog.Infof("test clients stopped")
	disableRandomDelay()
	plog.Infof("random large delay disabled")
	// restore all nodehost instances and wait for long enough
	te.startNodeHostNodes(false)
	te.startDrummerNodes()
	te.randomDropPacket(false)
	plog.Infof("all nodes restarted")
	check(t, te.checkDrummerIsReady, 60)
	plog.Infof("going to check drummer cluster info")
	check(t, te.checkClusterState, 60)
	checker.Stop()
	checker.SaveAsJepsenLog(lcmlog)
	checker.SaveAsEDNLog(ednlog)
	plog.Infof("dumping memory profile to disk")
	saveHeapProfile("drummer_mem.pprof")
	te.ensureNodeHostNotPartitioned(t)
	plog.Infof("going to check nodehost cluster state")
	te.waitForNodeHosts()
	plog.Infof("clusters stable check done")
	check(t, te.checkNodeHostsSynced, 60)
	plog.Infof("sync check done")
	check(t, te.checkNodeHostSM, 60)
	plog.Infof("state machine check done")
	te.waitForDrummers()
	plog.Infof("drummer nodes stable check done")
	check(t, te.checkDrummersSynced, 30)
	plog.Infof("drummer sync check done")
	check(t, te.checkDrummerSM, 30)
	plog.Infof("check logdb entries")
	check(t, te.checkLogDBSynced, 30)
	plog.Infof("going to check in mem log sizes")
	check(t, te.checkRateLimiterState, 30)
	plog.Infof("total completed IO: %d", atomic.LoadUint64(&te.completedIO))
	plog.Infof("going to check cluster accessibility")
	//te.checkClustersAreAccessible(t)
	plog.Infof("cluster accessibility checked")
	plog.Infof("all done, test is going to return.")
}
