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
	"math/rand"
	"os"
	"runtime"
	"testing"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/logger"
	"github.com/lni/goutils/leaktest"
)

func runDrummerMonkeyTest(t *testing.T, appname string) {
	rand.Seed(int64(os.Getpid()))
	maxprocs := rand.Uint64()%8 + 3            // [3, 10]
	snapshotWorkerCount := rand.Uint64()%8 + 1 // [1, 8]
	workerCount := rand.Uint64()%4 + 1         // [1, 4]
	queueSz := rand.Uint64()%33 + 32           // [32, 64]
	plog.Infof("maxprocs %d", maxprocs)
	plog.Infof("snapshot worker count %d, worker count %d",
		snapshotWorkerCount, workerCount)
	plog.Infof("queue size: %d", queueSz)
	runtime.GOMAXPROCS(int(maxprocs))
	dragonboat.SetSnapshotWorkerCount(snapshotWorkerCount)
	dragonboat.SetStepWorkerCount(workerCount)
	dragonboat.SetApplyWorkerCount(workerCount)
	dragonboat.SetReceiveQueueLen(queueSz)
	dragonboat.SetPendingProposalShards(2)
	dragonboat.SetTaskBatchSize(8)
	dragonboat.SetIncomingProposalsMaxLen(64)
	dragonboat.SetIncomingReadIndexMaxLen(64)
	dragonboat.ApplyMonkeySettings()
	logger.GetLogger("dragonboat").SetLevel(logger.DEBUG)
	logger.GetLogger("transport").SetLevel(logger.DEBUG)
	drummerMonkeyTesting(t, appname)
}

func TestClusterCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "kvtest")
}

func TestConcurrentClusterCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "concurrentkv")
}

/*
func TestCPPKVCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "cpp-cppkvtest")
}*/
