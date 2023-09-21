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
	"context"
	"errors"

	pb "github.com/lni/drummer/v3/drummerpb"
)

var (
	// ErrInvalidRequest indicates the request can not be fulfilled as it is
	// regarded as invalid.
	ErrInvalidRequest = errors.New("invalid drummer request")
)

// AddDrummerServer adds a new drummer node with specified replicaID and address
// to the Drummer shard.
func AddDrummerServer(ctx context.Context, client pb.DrummerClient,
	replicaID uint64, address string) (*pb.Empty, error) {
	req := &pb.DrummerConfigRequest{
		ReplicaId: replicaID,
		Address:   address,
	}
	return client.AddDrummerServer(ctx, req)
}

// RemoveDrummerServer removes the specified node from the Drummer shard.
func RemoveDrummerServer(ctx context.Context, client pb.DrummerClient,
	replicaID uint64) (*pb.Empty, error) {
	req := &pb.DrummerConfigRequest{
		ReplicaId: replicaID,
	}
	return client.RemoveDrummerServer(ctx, req)
}

// SubmitCreateDrummerChange submits Drummer change used for defining shards.
func SubmitCreateDrummerChange(ctx context.Context, client pb.DrummerClient,
	shardID uint64, members []uint64, appName string) error {
	if shardID == 0 {
		panic("invalid shard ID")
	}
	if len(appName) == 0 {
		panic("empty app name")
	}
	if len(members) == 0 {
		panic("empty members")
	}
	change := pb.Change{
		Type:    pb.Change_CREATE,
		ShardId: shardID,
		Members: members,
		AppName: appName,
	}
	req, err := client.SubmitChange(ctx, &change)
	if err != nil {
		return err
	}
	if req.Code == pb.ChangeResponse_BOOTSTRAPPED {
		return ErrInvalidRequest
	}
	return nil
}

// GetShardCollection returns known shards from the Drummer server.
func GetShardCollection(ctx context.Context,
	client pb.DrummerClient) (*pb.ShardCollection, error) {
	return client.GetShards(ctx, &pb.Empty{})
}

// GetShardStates returns shard states known to the Drummer server.
func GetShardStates(ctx context.Context,
	client pb.DrummerClient, shards []uint64) (*pb.ShardStates, error) {
	req := &pb.ShardStateRequest{
		ShardIdList: shards,
	}
	return client.GetShardStates(ctx, req)
}

// GetNodeHostCollection returns nodehosts known to the Drummer.
func GetNodeHostCollection(ctx context.Context,
	client pb.DrummerClient) (*pb.NodeHostCollection, error) {
	return client.GetNodeHostCollection(ctx, &pb.Empty{})
}

// SubmitRegions submits regions info to the Drummer server.
func SubmitRegions(ctx context.Context,
	client pb.DrummerClient, region pb.Regions) error {
	_, err := client.SetRegions(ctx, &region)
	return err
}

// SubmitBootstrappped sets the bootstrapped flag on Drummer server.
func SubmitBootstrappped(ctx context.Context,
	client pb.DrummerClient) error {
	_, err := client.SetBootstrapped(ctx, &pb.Empty{})
	return err
}
