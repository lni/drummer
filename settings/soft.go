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

package settings

//
// Tuning configuration parameters here will impact the performance of your
// system. It will not corrupt your data. Only tune these parameters when
// you know what you are doing.
//
// To tune these parameters, place a json file named
// dragonboat-soft-settings.json in the current working directory of your
// dragonboat application, all fields in the json file will be applied to
// overwrite the default setting values. e.g. for a json file with the
// following content -
//
// {
//   "GetConnectedTimeoutSecond": 15,
//   "UnknownRegionName": "no-idea-region"
// }
//
// soft.GetConnectedTimeoutSecond will be 15,
// soft.UnknownRegionName will be "no-idea-region"
//
// The application need to be restarted to apply such configuration changes.
//

// Soft is the soft settings that can be changed after the deployment of a
// system.
var Soft = getSoftSettings()

type soft struct {
	// LocalRaftRequestTimeoutMs is the raft request timeout in millisecond.
	LocalRaftRequestTimeoutMs uint64
	// GetConnectedTimeoutSecond is the default timeout value in second when
	// trying to connect to a gRPC based server.
	GetConnectedTimeoutSecond uint64

	//
	// Drummer
	//

	// MaxDrummerServerMsgSize is the max size for drummer server message.
	MaxDrummerServerMsgSize uint64
	// UnknownRegionName is the name of the region
	UnknownRegionName string
	// MaxDrummerClientMsgSize is the max size for drummer client message.
	MaxDrummerClientMsgSize uint64
	// DrummerClientName defines the name of the built-in drummer client.
	DrummerClientName string
	// NodeHostInfoReportSecond defines how often in seconds nodehost report it
	// details to Drummer servers.
	NodeHostInfoReportSecond uint64
	// NodeHostTTL defines the number of seconds without any report from the
	// nodehost required to consider it as dead.
	NodeHostTTL uint64
	// NodeToStartMaxWait is the number of seconds allowed for a new node to stay
	// in the to start state. To start state is the stage when a node has been
	// added to the raft cluster but has not been confirmed to be launched and
	// running on its assigned nodehost.
	NodeToStartMaxWait uint64
	// DrummerLoopIntervalFactor defines how often Drummer need to examine all
	// NodeHost info reported to it measured by the number of nodehost info
	// report cycles.
	DrummerLoopIntervalFactor uint64
	// PersisentLogReportCycle defines how often local persisted log info need
	// to be reported to Drummer server. Each NodeHostInfoReportSecond is called
	// a cycle. PersisentLogReportCycle defines how often each nodehost need to
	// update Drummer servers in terms of NodeHostInfoReportSecond cycles.
	PersisentLogReportCycle uint64
}

func getSoftSettings() soft {
	return getDefaultSoftSettings()
}

const (
	LaunchDeadlineTick  uint64 = 24
	MaxMessageBatchSize uint64 = 64 * 1024 * 1024
)

func getDefaultSoftSettings() soft {
	NodeHostInfoReportSecond := uint64(20)
	return soft{
		LocalRaftRequestTimeoutMs: 10000,
		GetConnectedTimeoutSecond: 5,
		UnknownRegionName:         "UNKNOWN",
		DrummerClientName:         "drummer-client",
		MaxDrummerClientMsgSize:   256 * 1024 * 1024,
		MaxDrummerServerMsgSize:   256 * 1024 * 1024,
		NodeHostInfoReportSecond:  NodeHostInfoReportSecond,
		NodeHostTTL:               NodeHostInfoReportSecond * 3,
		NodeToStartMaxWait:        NodeHostInfoReportSecond * 12,
		DrummerLoopIntervalFactor: 1,
		PersisentLogReportCycle:   3,
	}
}
