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

package tests

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cockroachdb/pebble"
	pvfs "github.com/cockroachdb/pebble/vfs"

	"github.com/lni/dragonboat/v3/config"
	sm "github.com/lni/dragonboat/v3/statemachine"
	"github.com/lni/drummer/v3/kvpb"
	"github.com/lni/goutils/logutil"
	"github.com/lni/goutils/random"
)

const (
	appliedIndexKey    string = "disk_kv_applied_index"
	testDBDirName      string = "test_rocksdb_db_safe_to_delete"
	currentDBFilename  string = "current"
	updatingDBFilename string = "current.updating"
)

func DirExist(name string, fs config.IFS) (bool, error) {
	if name == "." || name == "/" {
		return true, nil
	}
	f, err := fs.OpenDir(name)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !s.IsDir() {
		panic("not a dir")
	}
	return true, nil
}

func MkdirAll(dir string, fs config.IFS) error {
	exist, err := DirExist(dir, fs)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}
	parent := fs.PathDir(dir)
	exist, err = DirExist(parent, fs)
	if err != nil {
		return err
	}
	if !exist {
		if err := MkdirAll(parent, fs); err != nil {
			return err
		}
	}
	return Mkdir(dir, fs)
}

func Mkdir(dir string, fs config.IFS) error {
	parent := fs.PathDir(dir)
	exist, err := DirExist(parent, fs)
	if err != nil {
		return err
	}
	if !exist {
		panic(fmt.Sprintf("%s doesn't exist when creating %s", parent, dir))
	}
	if err := fs.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return syncDir(parent, fs)
}

func syncDir(dir string, fs config.IFS) (err error) {
	if runtime.GOOS == "windows" {
		return nil
	}
	if dir == "." {
		return nil
	}
	f, err := fs.OpenDir(filepath.Clean(dir))
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	fileInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		panic("not a dir")
	}
	return f.Sync()
}

// PebbleFS is a wrapper struct that implements the pebble/vfs.FS interface.
type PebbleFS struct {
	fs config.IFS
}

// NewPebbleFS creates a new pebble/vfs.FS instance.
func NewPebbleFS(fs config.IFS) pvfs.FS {
	return &PebbleFS{fs}
}

// Create ...
func (p *PebbleFS) Create(name string) (pvfs.File, error) {
	return p.fs.Create(name)
}

// Link ...
func (p *PebbleFS) Link(oldname, newname string) error {
	return p.fs.Link(oldname, newname)
}

// Open ...
func (p *PebbleFS) Open(name string, opts ...pvfs.OpenOption) (pvfs.File, error) {
	f, err := p.fs.Open(name)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt.Apply(f)
	}
	return f, nil
}

// OpenDir ...
func (p *PebbleFS) OpenDir(name string) (pvfs.File, error) {
	return p.fs.OpenDir(name)
}

// Remove ...
func (p *PebbleFS) Remove(name string) error {
	return p.fs.Remove(name)
}

// RemoveAll ...
func (p *PebbleFS) RemoveAll(name string) error {
	return p.fs.RemoveAll(name)
}

// Rename ...
func (p *PebbleFS) Rename(oldname, newname string) error {
	return p.fs.Rename(oldname, newname)
}

// ReuseForWrite ...
func (p *PebbleFS) ReuseForWrite(oldname, newname string) (pvfs.File, error) {
	return p.fs.ReuseForWrite(oldname, newname)
}

// MkdirAll ...
func (p *PebbleFS) MkdirAll(dir string, perm os.FileMode) error {
	return p.fs.MkdirAll(dir, perm)
}

// Lock ...
func (p *PebbleFS) Lock(name string) (io.Closer, error) {
	return p.fs.Lock(name)
}

// List ...
func (p *PebbleFS) List(dir string) ([]string, error) {
	return p.fs.List(dir)
}

// Stat ...
func (p *PebbleFS) Stat(name string) (os.FileInfo, error) {
	return p.fs.Stat(name)
}

// PathBase ...
func (p *PebbleFS) PathBase(path string) string {
	return p.fs.PathBase(path)
}

// PathJoin ...
func (p *PebbleFS) PathJoin(elem ...string) string {
	return p.fs.PathJoin(elem...)
}

// PathDir ...
func (p *PebbleFS) PathDir(path string) string {
	return p.fs.PathDir(path)
}

type pebbledb struct {
	mu     sync.RWMutex
	db     *pebble.DB
	ro     *pebble.IterOptions
	wo     *pebble.WriteOptions
	syncwo *pebble.WriteOptions
	closed bool
	fs     config.IFS
}

func (r *pebbledb) lookup(query []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return nil, errors.New("db already closed")
	}
	val, closer, err := r.db.Get(query)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	if len(val) == 0 {
		return []byte(""), nil
	}
	buf := make([]byte, len(val))
	copy(buf, val)
	return buf, nil
}

func (r *pebbledb) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.db != nil {
		r.db.Close()
	}
}

func createDB(dbdir string, fs config.IFS) (*pebbledb, error) {
	ro := &pebble.IterOptions{}
	wo := &pebble.WriteOptions{Sync: false}
	syncwo := &pebble.WriteOptions{Sync: true}
	cache := pebble.NewCache(0)
	opts := &pebble.Options{
		MaxManifestFileSize: 1024 * 32,
		MemTableSize:        1024 * 32,
		Cache:               cache,
		FS:                  NewPebbleFS(fs),
	}
	if err := MkdirAll(dbdir, fs); err != nil {
		return nil, err
	}
	db, err := pebble.Open(dbdir, opts)
	if err != nil {
		return nil, err
	}
	cache.Unref()
	return &pebbledb{
		db:     db,
		ro:     ro,
		wo:     wo,
		syncwo: syncwo,
	}, nil
}

func isNewRun(dir string, fs config.IFS) bool {
	fp := fs.PathJoin(dir, currentDBFilename)
	if _, err := fs.Stat(fp); os.IsNotExist(err) {
		return true
	}
	return false
}

func getNodeDBDirName(clusterID uint64, nodeID uint64, fs config.IFS) string {
	part := fmt.Sprintf("%d_%d", clusterID, nodeID)
	dir, err := filepath.Abs(fs.PathJoin(testDBDirName, part))
	if err != nil {
		panic(err)
	}
	return dir
}

func getNewRandomDBDirName(dir string, fs config.IFS) string {
	part := "%d_%d"
	rn := random.LockGuardedRand.Uint64()
	ct := time.Now().UnixNano()
	return fs.PathJoin(dir, fmt.Sprintf(part, rn, ct))
}

func replaceCurrentDBFile(dir string, fs config.IFS) error {
	fp := fs.PathJoin(dir, currentDBFilename)
	tmpFp := fs.PathJoin(dir, updatingDBFilename)
	if err := fs.Rename(tmpFp, fp); err != nil {
		return err
	}
	return syncDir(dir, fs)
}

func saveCurrentDBDirName(dir string, dbdir string, fs config.IFS) error {
	h := md5.New()
	if _, err := h.Write([]byte(dbdir)); err != nil {
		return err
	}
	fp := fs.PathJoin(dir, updatingDBFilename)
	f, err := fs.Create(fp)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
		if err := syncDir(dir, fs); err != nil {
			panic(err)
		}
	}()
	if _, err := f.Write(h.Sum(nil)[:8]); err != nil {
		return err
	}
	if _, err := f.Write([]byte(dbdir)); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}

func getCurrentDBDirName(dir string, fs config.IFS) (string, error) {
	fp := fs.PathJoin(dir, currentDBFilename)
	f, err := fs.Open(fp)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	if len(data) <= 8 {
		panic("corrupted content")
	}
	crc := data[:8]
	content := data[8:]
	h := md5.New()
	if _, err := h.Write(content); err != nil {
		return "", err
	}
	if !bytes.Equal(crc, h.Sum(nil)[:8]) {
		panic("corrupted content with not matched crc")
	}
	return string(content), nil
}

func createNodeDataDir(dir string, fs config.IFS) error {
	parent := fs.PathDir(dir)
	if err := MkdirAll(dir, fs); err != nil {
		return err
	}
	return syncDir(parent, fs)
}

func cleanupNodeDataDir(dir string, fs config.IFS) error {
	if err := fs.RemoveAll(fs.PathJoin(dir, updatingDBFilename)); err != nil {
		return err
	}
	dbdir, err := getCurrentDBDirName(dir, fs)
	if err != nil {
		return err
	}
	dirs, err := fs.List(dir)
	if err != nil {
		return err
	}
	for _, v := range dirs {
		fp := fs.PathJoin(dir, v)
		fi, err := fs.Stat(fp)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			continue
		}
		fmt.Printf("dbdir %s, fi.name %s, dir %s\n", dbdir, fi.Name(), dir)
		toDelete := fs.PathJoin(dir, fi.Name())
		if toDelete != dbdir {
			fmt.Printf("removing %s\n", toDelete)
			if err := fs.RemoveAll(toDelete); err != nil {
				return err
			}
		}
	}
	return nil
}

// DiskKVTest is a state machine used for testing on disk kv.
type DiskKVTest struct {
	clusterID   uint64
	nodeID      uint64
	lastApplied uint64
	db          unsafe.Pointer
	closed      bool
	aborted     bool
	fs          config.IFS
}

// NewDiskKVTest creates a new disk kv test state machine.
func NewDiskKVTest(clusterID uint64, nodeID uint64) sm.IOnDiskStateMachine {
	d := &DiskKVTest{
		clusterID: clusterID,
		nodeID:    nodeID,
	}
	fmt.Printf("[DKVE] %s is being created\n", d.id())
	return d
}

func (d *DiskKVTest) id() string {
	id := logutil.DescribeNode(d.clusterID, d.nodeID)
	return fmt.Sprintf("%s %s", time.Now().Format("2006-01-02 15:04:05.000000"), id)
}

func (d *DiskKVTest) queryAppliedIndex(db *pebbledb) (uint64, error) {
	val, closer, err := db.db.Get([]byte(appliedIndexKey))
	if err != nil && err != pebble.ErrNotFound {
		fmt.Printf("[DKVE] %s failed to query applied index\n", d.id())
		return 0, err
	}
	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()
	if len(val) == 0 {
		fmt.Printf("[DKVE] %s does not have applied index stored yet\n", d.id())
		return 0, nil
	}
	return binary.LittleEndian.Uint64(val), nil
}

// SetTestFS sets the fs of the test SM.
func (d *DiskKVTest) SetTestFS(fs config.IFS) {
	if d.fs != nil {
		panic("d.fs is not nil")
	}
	d.fs = fs
}

// Open opens the state machine.
func (d *DiskKVTest) Open(stopc <-chan struct{}) (uint64, error) {
	fmt.Printf("[DKVE] %s is being opened\n", d.id())
	generateRandomDelay()
	if d.fs == nil {
		panic("d.fs not set")
	}
	dir := getNodeDBDirName(d.clusterID, d.nodeID, d.fs)
	if err := createNodeDataDir(dir, d.fs); err != nil {
		panic(err)
	}
	if d.fs == nil {
		panic("d.fs not set")
	}
	var dbdir string
	if !isNewRun(dir, d.fs) {
		if err := cleanupNodeDataDir(dir, d.fs); err != nil {
			return 0, err
		}
		var err error
		dbdir, err = getCurrentDBDirName(dir, d.fs)
		if err != nil {
			return 0, err
		}
		if _, err := d.fs.Stat(dbdir); err != nil {
			if os.IsNotExist(err) {
				panic("db dir unexpectedly deleted")
			}
		}
		fmt.Printf("[DKVE] %s being re-opened at %s\n", d.id(), dbdir)
	} else {
		fmt.Printf("[DKVE] %s doing a new run\n", d.id())
		dbdir = getNewRandomDBDirName(dir, d.fs)
		if err := saveCurrentDBDirName(dir, dbdir, d.fs); err != nil {
			return 0, err
		}
		if err := replaceCurrentDBFile(dir, d.fs); err != nil {
			return 0, err
		}
	}
	fmt.Printf("[DKVE] %s going to create db at %s\n", d.id(), dbdir)
	db, err := createDB(dbdir, d.fs)
	if err != nil {
		fmt.Printf("[DKVE] %s failed to create db\n", d.id())
		return 0, err
	}
	fmt.Printf("[DKVE] %s returned from create db\n", d.id())
	atomic.SwapPointer(&d.db, unsafe.Pointer(db))
	appliedIndex, err := d.queryAppliedIndex(db)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[DKVE] %s opened its disk sm, index %d\n", d.id(), appliedIndex)
	d.lastApplied = appliedIndex
	return appliedIndex, nil
}

// Lookup queries the state machine.
func (d *DiskKVTest) Lookup(key interface{}) (interface{}, error) {
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	if db != nil {
		v, err := db.lookup(key.([]byte))
		if err == nil && d.closed {
			panic("lookup returned valid result when DiskKVTest is already closed")
		}
		if err == pebble.ErrNotFound {
			return v, nil
		}
		return v, err
	}
	return nil, errors.New("db closed")
}

// Update updates the state machine.
func (d *DiskKVTest) Update(ents []sm.Entry) ([]sm.Entry, error) {
	if d.aborted {
		panic("update() called after abort set to true")
	}
	if d.closed {
		panic("update called after Close()")
	}
	generateRandomDelay()
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	wb := db.db.NewBatch()
	defer wb.Close()
	for idx, e := range ents {
		dataKv := &kvpb.PBKV{}
		if err := dataKv.Unmarshal(e.Cmd); err != nil {
			panic(err)
		}
		key := dataKv.GetKey()
		val := dataKv.GetVal()
		wb.Set([]byte(key), []byte(val), db.syncwo)
		ents[idx].Result = sm.Result{Value: uint64(len(ents[idx].Cmd))}
	}
	idx := make([]byte, 8)
	binary.LittleEndian.PutUint64(idx, ents[len(ents)-1].Index)
	wb.Set([]byte(appliedIndexKey), idx, db.syncwo)
	fmt.Printf("[DKVE] %s applied index recorded as %d\n", d.id(), ents[len(ents)-1].Index)
	if err := db.db.Apply(wb, db.syncwo); err != nil {
		return nil, err
	}
	if d.lastApplied >= ents[len(ents)-1].Index {
		fmt.Printf("[DKVE] %s last applied not moving forward %d,%d\n",
			d.id(), ents[len(ents)-1].Index, d.lastApplied)
		panic("lastApplied not moving forward")
	}
	d.lastApplied = ents[len(ents)-1].Index
	return ents, nil
}

// Sync synchronizes state machine's in-core state with that on disk.
func (d *DiskKVTest) Sync() error {
	if d.aborted {
		panic("update() called after abort set to true")
	}
	if d.closed {
		panic("update called after Close()")
	}
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	wb := db.db.NewBatch()
	defer wb.Close()
	wb.Set([]byte("dummy-key"), []byte("dummy-value"), db.syncwo)
	return db.db.Apply(wb, db.syncwo)
}

type diskKVCtx struct {
	db       *pebbledb
	snapshot *pebble.Snapshot
}

// PrepareSnapshot prepares snapshotting.
func (d *DiskKVTest) PrepareSnapshot() (interface{}, error) {
	if d.closed {
		panic("prepare snapshot called after Close()")
	}
	if d.aborted {
		panic("prepare snapshot called after abort")
	}
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	return &diskKVCtx{
		db:       db,
		snapshot: db.db.NewSnapshot(),
	}, nil
}

func iteratorIsValid(iter *pebble.Iterator) bool {
	return iter.Valid()
}

func (d *DiskKVTest) saveToWriter(db *pebbledb,
	ss *pebble.Snapshot, w io.Writer) error {
	iter := ss.NewIter(db.ro)
	defer iter.Close()
	var dataMap sync.Map
	values := make([]*kvpb.PBKV, 0)
	for iter.First(); iteratorIsValid(iter); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		dataMap.Store(string(key), string(val))
	}
	toList := func(k, v interface{}) bool {
		kv := &kvpb.PBKV{
			Key: k.(string),
			Val: v.(string),
		}
		values = append(values, kv)
		return true
	}
	dataMap.Range(toList)
	sort.Slice(values, func(i, j int) bool {
		return strings.Compare(values[i].Key, values[j].Key) < 0
	})
	count := uint64(len(values))
	fmt.Printf("[DKVE] %s have %d pairs of KV\n", d.id(), count)
	sz := make([]byte, 8)
	binary.LittleEndian.PutUint64(sz, count)
	if _, err := w.Write(sz); err != nil {
		return err
	}
	for _, dataKv := range values {
		if dataKv.Key == appliedIndexKey {
			idx := binary.LittleEndian.Uint64([]byte(dataKv.Val))
			fmt.Printf("[DKVE] %s saving appliedIndexKey as %d\n", d.id(), idx)
		}
		data, err := dataKv.Marshal()
		if err != nil {
			panic(err)
		}
		binary.LittleEndian.PutUint64(sz, uint64(len(data)))
		if _, err := w.Write(sz); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// SaveSnapshot saves the state machine state.
func (d *DiskKVTest) SaveSnapshot(ctx interface{},
	w io.Writer, done <-chan struct{}) error {
	if d.closed {
		panic("prepare snapshot called after Close()")
	}
	if d.aborted {
		panic("prepare snapshot called after abort")
	}
	delay := getLargeRandomDelay(d.clusterID)
	fmt.Printf("random delay %d ms\n", delay)
	for delay > 0 {
		delay -= 10
		time.Sleep(10 * time.Millisecond)
		select {
		case <-done:
			return sm.ErrSnapshotStopped
		default:
		}
	}
	rsz := uint64(1024 * 1024 * 6)
	rubbish := make([]byte, rsz)
	for i := 0; i < 512; i++ {
		idx := random.LockGuardedRand.Uint64() % rsz
		rubbish[idx] = byte(random.LockGuardedRand.Uint64())
	}
	_, err := w.Write(rubbish)
	if err != nil {
		return err
	}
	ctxdata := ctx.(*diskKVCtx)
	db := ctxdata.db
	db.mu.RLock()
	defer db.mu.RUnlock()
	ss := ctxdata.snapshot
	defer ss.Close()
	return d.saveToWriter(db, ss, w)
}

// RecoverFromSnapshot recovers the state machine state from snapshot.
func (d *DiskKVTest) RecoverFromSnapshot(r io.Reader,
	done <-chan struct{}) error {
	if d.closed {
		panic("recover from snapshot called after Close()")
	}
	delay := getLargeRandomDelay(d.clusterID)
	fmt.Printf("random delay %d ms\n", delay)
	for delay > 0 {
		delay -= 10
		time.Sleep(10 * time.Millisecond)
		select {
		case <-done:
			d.aborted = true
			return sm.ErrSnapshotStopped
		default:
		}
	}
	rubbish := make([]byte, 1024*1024*6)
	if _, err := io.ReadFull(r, rubbish); err != nil {
		return err
	}
	dir := getNodeDBDirName(d.clusterID, d.nodeID, d.fs)
	dbdir := getNewRandomDBDirName(dir, d.fs)
	oldDirName, err := getCurrentDBDirName(dir, d.fs)
	if err != nil {
		return err
	}
	fmt.Printf("[DKVE] %s is creating a tmp db at %s\n", d.id(), dbdir)
	db, err := createDB(dbdir, d.fs)
	if err != nil {
		return err
	}
	sz := make([]byte, 8)
	if _, err := io.ReadFull(r, sz); err != nil {
		return err
	}
	total := binary.LittleEndian.Uint64(sz)
	fmt.Printf("[DKVE] %s recovering from a snapshot with %d pairs of KV\n", d.id(), total)
	wb := db.db.NewBatch()
	defer wb.Close()
	for i := uint64(0); i < total; i++ {
		if _, err := io.ReadFull(r, sz); err != nil {
			return err
		}
		toRead := binary.LittleEndian.Uint64(sz)
		data := make([]byte, toRead)
		if _, err := io.ReadFull(r, data); err != nil {
			return err
		}
		dataKv := &kvpb.PBKV{}
		if err := dataKv.Unmarshal(data); err != nil {
			panic(err)
		}
		if dataKv.Key == appliedIndexKey {
			v := binary.LittleEndian.Uint64([]byte(dataKv.Val))
			if err != nil {
				panic(err)
			}
			fmt.Printf("[DKVE] %s recovering appliedIndexKey to %d\n", d.id(), v)
		}
		wb.Set([]byte(dataKv.Key), []byte(dataKv.Val), db.syncwo)
	}
	if err := db.db.Apply(wb, db.syncwo); err != nil {
		return err
	}
	if err := saveCurrentDBDirName(dir, dbdir, d.fs); err != nil {
		return err
	}
	if err := replaceCurrentDBFile(dir, d.fs); err != nil {
		return err
	}
	fmt.Printf("[DKVE] %s replaced db %s with %s\n", d.id(), oldDirName, dbdir)
	newLastApplied, err := d.queryAppliedIndex(db)
	if err != nil {
		panic(err)
	}
	// when d.lastApplied == newLastApplied, it probably means there were some
	// dummy entries or membership change entries as part of the new snapshot
	// that never reached the SM and thus never moved the last applied index
	// in the SM snapshot.
	if d.lastApplied > newLastApplied {
		fmt.Printf("[DKVE] %s last applied in snapshot not moving forward %d,%d\n",
			d.id(), d.lastApplied, newLastApplied)
		panic("last applied not moving forward")
	}
	d.lastApplied = newLastApplied
	old := (*pebbledb)(atomic.SwapPointer(&d.db, unsafe.Pointer(db)))
	if old != nil {
		old.close()
	}
	fmt.Printf("[DKVE] %s to delete olddb at %s\n", d.id(), oldDirName)
	parent := d.fs.PathDir(oldDirName)
	if err := d.fs.RemoveAll(oldDirName); err != nil {
		return err
	}
	return syncDir(parent, d.fs)
}

// Close closes the state machine.
func (d *DiskKVTest) Close() error {
	fmt.Printf("[DKVE] %s called close\n", d.id())
	db := (*pebbledb)(atomic.SwapPointer(&d.db, unsafe.Pointer(nil)))
	if db != nil {
		d.closed = true
		db.close()
	} else {
		if d.closed {
			panic("close called twice")
		}
	}
	return nil
}

// GetHash returns a hash value representing the state of the state machine.
func (d *DiskKVTest) GetHash() (uint64, error) {
	fmt.Printf("[DKVE] %s called GetHash\n", d.id())
	h := md5.New()
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	ss := db.db.NewSnapshot()
	defer ss.Close()
	db.mu.RLock()
	defer db.mu.RUnlock()
	if err := d.saveToWriter(db, ss, h); err != nil {
		return 0, err
	}
	md5sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(md5sum[:8]), nil
}
