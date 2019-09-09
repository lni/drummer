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
	runtime.GOMAXPROCS(10)
	dragonboat.SetSnapshotWorkerCount(8)
	dragonboat.SetWorkerCount(4)
	dragonboat.SetTaskWorkerCount(4)
	dragonboat.SetIncomingProposalsMaxLen(64)
	dragonboat.SetIncomingReadIndexMaxLen(64)
	dragonboat.SetReceiveQueueLen(64)
	dragonboat.ApplyMonkeySettings()
	rand.Seed(int64(os.Getpid()))
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

func TestOnDiskClusterCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "diskkv")
}

/*
func TestCPPKVCanSurviveDrummerMonkeyPlay(t *testing.T) {
	defer leaktest.AfterTest(t)()
	runDrummerMonkeyTest(t, "cpp-cppkvtest")
}*/
