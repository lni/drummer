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

// +build dragonboat_monkeytest

package drummer

import (
	"flag"
	"math/rand"
	"runtime"
	"testing"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/logger"
	"github.com/lni/goutils/leaktest"
)

var (
	port   = flag.Int("port", 24000, "base port")
	seed   = flag.Int64("seed", 0, "seed for the rng")
	silent = flag.Bool("silent", false, "less verbose logging")
)

type testOption struct {
	seed                int64
	port                uint64
	maxProcs            uint64
	snapshotWorkerCount uint64
	workerCount         uint64
	queueLength         uint64
	silent              bool
}

func getTestOption() *testOption {
	flag.Parse()
	return &testOption{
		seed:                *seed,
		port:                uint64(*port),
		silent:              *silent,
		maxProcs:            rand.Uint64()%8 + 3,   // [3, 10]
		snapshotWorkerCount: rand.Uint64()%8 + 1,   // [1, 8]
		workerCount:         rand.Uint64()%4 + 1,   // [1, 4]
		queueLength:         rand.Uint64()%33 + 32, // [32, 64]
	}
}

func runDrummerMonkeyTest(t *testing.T, name string) {
	to := getTestOption()
	rand.Seed(to.seed)
	plog.Infof("maxProcs %d", to.maxProcs)
	plog.Infof("snapshot worker count %d, worker count %d",
		to.snapshotWorkerCount, to.workerCount)
	plog.Infof("queue size: %d", to.queueLength)
	runtime.GOMAXPROCS(int(to.maxProcs))
	dragonboat.SetSnapshotWorkerCount(to.snapshotWorkerCount)
	dragonboat.SetApplyWorkerCount(to.workerCount)
	dragonboat.SetReceiveQueueLen(to.queueLength)
	dragonboat.SetPendingProposalShards(2)
	dragonboat.SetTaskBatchSize(8)
	dragonboat.SetIncomingProposalsMaxLen(64)
	dragonboat.SetIncomingReadIndexMaxLen(64)
	dragonboat.ApplyMonkeySettings()
	if to.silent {
		plog.Infof("silent mode, most logging will be disabled")
		logger.GetLogger("dragonboat").SetLevel(logger.ERROR)
		logger.GetLogger("transport").SetLevel(logger.ERROR)
		logger.GetLogger("raft").SetLevel(logger.ERROR)
		logger.GetLogger("rsm").SetLevel(logger.ERROR)
		logger.GetLogger("logdb").SetLevel(logger.ERROR)
		logger.GetLogger("drummer/client").SetLevel(logger.ERROR)
	}
	drummerMonkeyTesting(t, to, name)
}

func TestClusterCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "kvtest")
}

func TestConcurrentClusterCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "concurrentkv")
}

func TestOnDiskClusterCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "diskkv")
}
