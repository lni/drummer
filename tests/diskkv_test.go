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

package tests

import (
	"bytes"
	"encoding/binary"
	"path"
	"testing"

	"github.com/cockroachdb/errors/oserror"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/lni/drummer/v3/kv"
)

func TestDBCanBeCreatedAndUsed(t *testing.T) {
	fs := dragonboat.GetTestFS()
	dbdir := "rocksdb_db_test_safe_to_delete"
	defer fs.RemoveAll(dbdir)
	db, err := createDB(dbdir, fs)
	if err != nil {
		t.Fatalf("failed to create db %+v", err)
	}
	defer db.close()
	key := []byte("test-key")
	val := []byte("test-val")
	if err := db.db.Set(key, val, db.wo); err != nil {
		t.Fatalf("failed to put kv %v", err)
	}
	result, err := db.lookup(key)
	if err != nil {
		t.Fatalf("lookup failed %v", err)
	}
	if !bytes.Equal(result, val) {
		t.Fatalf("result changed")
	}
}

func TestIsNewRun(t *testing.T) {
	fs := dragonboat.GetTestFS()
	dbdir := "rocksdb_db_test_safe_to_delete"
	defer fs.RemoveAll(dbdir)
	if err := fs.MkdirAll(dbdir, 0755); err != nil {
		t.Fatalf("%v", err)
	}
	if !isNewRun(dbdir, fs) {
		t.Errorf("not a new run")
	}
	f, err := fs.Create(fs.PathJoin(dbdir, currentDBFilename))
	if err != nil {
		t.Fatalf("failed to create the current db file")
	}
	f.Close()
	if isNewRun(dbdir, fs) {
		t.Errorf("still considered as a new run")
	}
}

func TestGetNodeDBDirName(t *testing.T) {
	fs := dragonboat.GetTestFS()
	names := make(map[string]struct{})
	for c := uint64(0); c < 128; c++ {
		for n := uint64(0); n < 128; n++ {
			name := getNodeDBDirName(c, n, fs)
			names[name] = struct{}{}
		}
	}
	if len(names) != 128*128 {
		t.Errorf("dup found")
	}
}

func TestGetNewRandomDBDirName(t *testing.T) {
	fs := dragonboat.GetTestFS()
	names := make(map[string]struct{})
	for c := uint64(0); c < 128; c++ {
		for n := uint64(0); n < 128; n++ {
			name := getNodeDBDirName(c, n, fs)
			dbdir := getNewRandomDBDirName(name, fs)
			names[dbdir] = struct{}{}
		}
	}
	if len(names) != 128*128 {
		t.Errorf("dup found")
	}
}

func TestSaveCurrentDBDirName(t *testing.T) {
	fs := dragonboat.GetTestFS()
	dbdir := "rocksdb_db_test_safe_to_delete"
	if err := fs.MkdirAll(dbdir, 0755); err != nil {
		t.Fatalf("%v", err)
	}
	defer fs.RemoveAll(dbdir)
	content := "content"
	if err := saveCurrentDBDirName(dbdir, content, fs); err != nil {
		t.Fatalf("failed to save current file %v", err)
	}
	if _, err := fs.Stat(fs.PathJoin(dbdir, updatingDBFilename)); oserror.IsNotExist(err) {
		t.Fatalf("file not exist")
	}
	if !isNewRun(dbdir, fs) {
		t.Errorf("suppose to be a new run")
	}
	if err := replaceCurrentDBFile(dbdir, fs); err != nil {
		t.Errorf("failed to rename the current db file %v", err)
	}
	if isNewRun(dbdir, fs) {
		t.Errorf("still a new run")
	}
	result, err := getCurrentDBDirName(dbdir, fs)
	if err != nil {
		t.Fatalf("failed to get current db dir name %v", err)
	}
	if result != content {
		t.Errorf("content changed")
	}
}

func TestCleanupNodeDataDir(t *testing.T) {
	fs := dragonboat.GetTestFS()
	dbdir := "rocksdb_db_test_safe_to_delete"
	if err := fs.MkdirAll(dbdir, 0755); err != nil {
		t.Fatalf("%v", err)
	}
	defer fs.RemoveAll(dbdir)
	toKeep := "dir_to_keep"
	if err := fs.MkdirAll(fs.PathJoin(dbdir, toKeep), 0755); err != nil {
		t.Fatalf("failed to create dir %v", err)
	}
	if err := fs.MkdirAll(fs.PathJoin(dbdir, "d1"), 0755); err != nil {
		t.Fatalf("failed to create dir %v", err)
	}
	if err := fs.MkdirAll(fs.PathJoin(dbdir, "d2"), 0755); err != nil {
		t.Fatalf("failed to create dir %v", err)
	}
	if err := saveCurrentDBDirName(dbdir, fs.PathJoin(dbdir, toKeep), fs); err != nil {
		t.Fatalf("failed to save current file %v", err)
	}
	if err := replaceCurrentDBFile(dbdir, fs); err != nil {
		t.Errorf("failed to rename the current db file %v", err)
	}
	if err := cleanupNodeDataDir(dbdir, fs); err != nil {
		t.Errorf("cleanup failed %v", err)
	}
	tests := []struct {
		name  string
		exist bool
	}{
		{dbdir, true},
		{path.Join(dbdir, toKeep), true},
		{path.Join(dbdir, "d1"), false},
		{path.Join(dbdir, "d2"), false},
	}
	for idx, tt := range tests {
		if _, err := fs.Stat(tt.name); oserror.IsNotExist(err) {
			if tt.exist {
				t.Errorf("unexpected cleanup result %d", idx)
			}
		}
	}
}

func removeAllDBDir(fs config.IFS) {
	fs.RemoveAll(testDBDirName)
}

func runDiskKVTest(t *testing.T, f func(t *testing.T, odsm sm.IOnDiskStateMachine)) {
	fs := dragonboat.GetTestFS()
	clusterID := uint64(128)
	nodeID := uint64(256)
	removeAllDBDir(fs)
	defer removeAllDBDir(fs)
	odsm := NewDiskKVTest(clusterID, nodeID)
	f(t, odsm)
}

func TestDiskKVCanBeOpened(t *testing.T) {
	fs := dragonboat.GetTestFS()
	tf := func(t *testing.T, odsm sm.IOnDiskStateMachine) {
		defer odsm.Close()
		odsm.(*DiskKVTest).SetTestFS(fs)
		idx, err := odsm.Open(nil)
		if err != nil {
			t.Fatalf("failed to open %v", err)
		}
		if idx != 0 {
			t.Fatalf("idx %d", idx)
		}
		_, err = odsm.Lookup([]byte(appliedIndexKey))
		if err != nil {
			t.Fatalf("lookup failed %v", err)
		}
	}
	runDiskKVTest(t, tf)
}

func TestDiskKVCanBeUpdated(t *testing.T) {
	fs := dragonboat.GetTestFS()
	tf := func(t *testing.T, odsm sm.IOnDiskStateMachine) {
		defer odsm.Close()
		odsm.(*DiskKVTest).SetTestFS(fs)
		idx, err := odsm.Open(nil)
		if err != nil {
			t.Fatalf("failed to open %v", err)
		}
		if idx != 0 {
			t.Fatalf("idx %d", idx)
		}
		pair1 := &kv.KV{
			Key: "test-key1",
			Val: "test-val1",
		}
		pair2 := &kv.KV{
			Key: "test-key2",
			Val: "test-val2",
		}
		data1, err := pair1.MarshalBinary()
		if err != nil {
			panic(err)
		}
		data2, err := pair2.MarshalBinary()
		if err != nil {
			panic(err)
		}
		ents := []sm.Entry{
			{Index: 1, Cmd: data1},
			{Index: 2, Cmd: data2},
		}
		if _, err := odsm.Update(ents); err != nil {
			t.Fatalf("%v", err)
		}
		if err := odsm.Sync(); err != nil {
			t.Fatalf("sync failed %v", err)
		}
		result, err := odsm.Lookup([]byte(appliedIndexKey))
		if err != nil {
			t.Fatalf("lookup failed %v", err)
		}
		idx = binary.LittleEndian.Uint64(result.([]byte))
		if idx != 2 {
			t.Errorf("last applied %d, want 2", idx)
		}
		result, err = odsm.Lookup([]byte("test-key1"))
		if err != nil {
			t.Fatalf("lookup failed %v", err)
		}
		if !bytes.Equal(result.([]byte), []byte("test-val1")) {
			t.Errorf("value not set")
		}
	}
	runDiskKVTest(t, tf)
}

func TestDiskKVSnapshot(t *testing.T) {
	fs := dragonboat.GetTestFS()
	tf := func(t *testing.T, odsm sm.IOnDiskStateMachine) {
		odsm.(*DiskKVTest).disableSnapshotAbort = true
		odsm.(*DiskKVTest).SetTestFS(fs)
		idx, err := odsm.Open(nil)
		if err != nil {
			t.Fatalf("failed to open %v", err)
		}
		defer odsm.Close()
		if idx != 0 {
			t.Fatalf("idx %d", idx)
		}
		pair1 := &kv.KV{
			Key: "test-key1",
			Val: "test-val1",
		}
		pair2 := &kv.KV{
			Key: "test-key2",
			Val: "test-val2",
		}
		pair3 := &kv.KV{
			Key: "test-key3",
			Val: "test-val3",
		}
		data1, err := pair1.MarshalBinary()
		if err != nil {
			panic(err)
		}
		data2, err := pair2.MarshalBinary()
		if err != nil {
			panic(err)
		}
		data3, err := pair3.MarshalBinary()
		if err != nil {
			panic(err)
		}
		ents := []sm.Entry{
			{Index: 1, Cmd: data1},
			{Index: 2, Cmd: data2},
		}
		if _, err := odsm.Update(ents); err != nil {
			t.Fatalf("%v", err)
		}
		hash1, err := odsm.(sm.IHash).GetHash()
		if err != nil {
			t.Fatalf("failed to get hash %v", err)
		}
		buf := bytes.NewBuffer(make([]byte, 0, 128))
		ctx, err := odsm.PrepareSnapshot()
		if err != nil {
			t.Fatalf("prepare snapshot failed %v", err)
		}
		err = odsm.SaveSnapshot(ctx, buf, nil)
		if err != nil {
			t.Fatalf("create snapshot failed %v", err)
		}
		if _, err := odsm.Update([]sm.Entry{{Index: 3, Cmd: data3}}); err != nil {
			t.Fatalf("%v", err)
		}
		result, err := odsm.Lookup([]byte("test-key3"))
		if err != nil {
			t.Fatalf("lookup failed %v", err)
		}
		if !bytes.Equal(result.([]byte), []byte("test-val3")) {
			t.Errorf("value not set")
		}
		hash2, err := odsm.(sm.IHash).GetHash()
		if err != nil {
			t.Fatalf("failed to get hash %v", err)
		}
		if hash1 == hash2 {
			t.Errorf("hash doesn't change")
		}
		reader := bytes.NewBuffer(buf.Bytes())
		odsm2 := NewDiskKVTest(1024, 1024)
		odsm2.(*DiskKVTest).SetTestFS(fs)
		idx, err = odsm2.Open(nil)
		if err != nil {
			t.Fatalf("failed to open %v", err)
		}
		if idx != 0 {
			t.Fatalf("idx %d", idx)
		}
		if err := odsm2.RecoverFromSnapshot(reader, nil); err != nil {
			t.Fatalf("recover from snapshot failed %v", err)
		}
		hash3, err := odsm2.(sm.IHash).GetHash()
		if err != nil {
			t.Fatalf("failed to get hash %v", err)
		}
		if hash3 != hash1 {
			t.Errorf("hash changed")
		}
		result, err = odsm2.Lookup([]byte("test-key3"))
		if err != nil {
			t.Fatalf("lookup failed %v", err)
		}
		if len(result.([]byte)) > 0 {
			t.Fatalf("test-key3 still available in the db")
		}
		odsm2.Close()
		result, err = odsm2.Lookup([]byte("test-key3"))
		if err == nil {
			t.Fatalf("lookup allowed after close")
		}
		if result != nil {
			t.Fatalf("returned something %v", result)
		}
	}
	runDiskKVTest(t, tf)
}
