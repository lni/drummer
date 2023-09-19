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
	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/lni/drummer/v3/tests"
)

type pluginDetails struct {
	createNativeStateMachine     func(uint64, uint64) sm.IStateMachine
	createConcurrentStateMachine func(uint64, uint64) sm.IConcurrentStateMachine
	createOnDiskStateMachine     func(uint64, uint64) sm.IOnDiskStateMachine
}

func (pd *pluginDetails) isRegularStateMachine() bool {
	return pd.createNativeStateMachine != nil
}

func (pd *pluginDetails) isConcurrentStateMachine() bool {
	return pd.createConcurrentStateMachine != nil
}

func (pd *pluginDetails) isOnDiskStateMachine() bool {
	return pd.createOnDiskStateMachine != nil
}

func getPluginMap(path string) map[string]pluginDetails {
	result := make(map[string]pluginDetails)
	result["kvtest"] = pluginDetails{createNativeStateMachine: tests.NewKVTest}
	result["concurrentkv"] = pluginDetails{createConcurrentStateMachine: tests.NewConcurrentKVTest}
	result["diskkv"] = pluginDetails{createOnDiskStateMachine: tests.NewDiskKVTest}
	return result
}
