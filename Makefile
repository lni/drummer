# Copyright 2017-2019 Lei Ni (nilei81@gmail.com)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

OS := $(shell uname)
# the location of this Makefile
PKGROOT=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
# name of the package
PKGNAME=github.com/lni/drummer/v3

ifeq ($(DRAGONBOAT_LOGDB),leveldb)
$(info using leveldb based log storage)
GOCMD=go
LOGDB_TAG=dragonboat_leveldb_test
else ifeq ($(DRAGONBOAT_LOGDB),pebble)
GOCMD=go
LOGDB_TAG=dragonboat_pebble_test
else ifeq ($(DRAGONBOAT_LOGDB),custom)
$(info using custom lodb)
GOCMD=go
LOGDB_TAG=dragonboat_no_rocksdb
else ifeq ($(DRAGONBOAT_LOGDB),)
$(info using rocksdb based log storage)
ROCKSDB_MAJOR_VER=5
ROCKSDB_MINOR_VER=13
ROCKSDB_PATCH_VER=4
ROCKSDB_VER ?= $(ROCKSDB_MAJOR_VER).$(ROCKSDB_MINOR_VER).$(ROCKSDB_PATCH_VER)

ifeq ($(OS),Darwin)
ROCKSDB_SO_FILE=librocksdb.$(ROCKSDB_MAJOR_VER).dylib
else ifeq ($(OS),Linux)
ROCKSDB_SO_FILE=librocksdb.so.$(ROCKSDB_MAJOR_VER)
else
$(error OS type $(OS) not supported)
endif

ROCKSDB_INC_PATH ?=
ROCKSDB_LIB_PATH ?=
# figure out where is the rocksdb installation
# supported gorocksdb version in ./build/lib?
ifeq ($(ROCKSDB_LIB_PATH),)
ifeq ($(ROCKSDB_INC_PATH),)
ifneq ($(wildcard $(PKGROOT)/build/lib/$(ROCKSDB_SO_FILE)),)
ifneq ($(wildcard $(PKGROOT)/build/include/rocksdb/c.h),)
$(info rocksdb lib found at $(PKGROOT)/build/lib/$(ROCKSDB_SO_FILE))
ROCKSDB_LIB_PATH=$(PKGROOT)/build/lib
ROCKSDB_INC_PATH=$(PKGROOT)/build/include
endif
endif
endif
endif

# in /usr/local/lib?
ifeq ($(ROCKSDB_LIB_PATH),)
ifeq ($(ROCKSDB_INC_PATH),)
ifneq ($(wildcard /usr/local/lib/$(ROCKSDB_SO_FILE)),)
ifneq ($(wildcard /usr/local/include/rocksdb/c.h),)
$(info rocksdb lib found at /usr/local/lib/$(ROCKSDB_SO_FILE))
ROCKSDB_LIB_PATH=/usr/local/lib
endif
endif
endif
endif

# by default, shared rocksdb lib is used. when using the static rocksdb lib,
# you may need to add
# -lbz2 -lsnappy -lz -llz4
ifeq ($(ROCKSDB_LIB_PATH),)
CDEPS_LDFLAGS=-lrocksdb
else
CDEPS_LDFLAGS=-L$(ROCKSDB_LIB_PATH) -lrocksdb
endif

ifneq ($(ROCKSDB_INC_PATH),)
CGO_CXXFLAGS=CGO_CFLAGS="-I$(ROCKSDB_INC_PATH)"
endif

CGO_LDFLAGS=CGO_LDFLAGS="$(CDEPS_LDFLAGS)"
GOCMD=$(CGO_LDFLAGS) $(CGO_CXXFLAGS) go
else
$(error LOGDB type $(DRAGONBOAT_LOGDB) not supported)
endif

ifeq ($(RACE),1)
RACE_DETECTOR_FLAG=-race
$(warning "data race detector enabled, this is a DEBUG build")
endif

VERBOSE ?= -v
ifeq ($(VERBOSE),)
GO=@$(GOCMD)
else
GO=$(GOCMD)
endif

ifneq ($(TEST_TO_RUN),)
$(info Running selected tests $(TEST_TO_RUN))
SELECTED_TEST_OPTION=-run $(TEST_TO_RUN)
endif

BUILD_TEST_ONLY=-c -o test.bin
GOBUILDTAGVALS+=$(LOGDB_TAG)
GOBUILDTAGS="$(GOBUILDTAGVALS)"
TESTTAGVALS+=$(GOBUILDTAGVALS)
TESTTAGVALS+=$(LOGDB_TEST_BUILDTAGS)
TESTTAGS="$(TESTTAGVALS)"

DRUMMER_MONKEY_TESTING_BIN=drummer-monkey-testing
PORCUPINE_CHECKER_BIN=porcupine-checker-bin
DRUMMER_MONKEY_TEST_BUILDTAGS=dragonboat_monkeytest

$(DRUMMER_MONKEY_TESTING_BIN):
	$(GO) test $(RACE_DETECTOR_FLAG) $(VERBOSE) \
		-tags="$(DRUMMER_MONKEY_TEST_BUILDTAGS)" -c -o $@ $(PKGNAME)
drummer-monkey-test-bin:$(DRUMMER_MONKEY_TESTING_BIN)

$(PORCUPINE_CHECKER_BIN):
	$(GO) build -o $@ $(VERBOSE) $(PKGNAME)/lcm/checker
porcupine-checker: $(PORCUPINE_CHECKER_BIN)

TEST_OPTIONS=test -tags=$(TESTTAGS) -count=1 $(VERBOSE) \
  $(RACE_DETECTOR_FLAG) $(SELECTED_TEST_OPTION)

GOTEST=$(GO) $(TEST_OPTIONS)
test:
	$(GOTEST) $(PKGNAME)

test-slow-drummer: TESTTAGVALS+=$(DRUMMER_SLOW_TEST_BUILDTAGS)
test-slow-drummer:
	$(GOTEST) -o $(DRUMMER_MONKEY_TESTING_BIN) -c $(PKGNAME)
	./$(DRUMMER_MONKEY_TESTING_BIN) -test.v -test.timeout 9999s

test-monkey-drummer: TESTTAGVALS+=$(DRUMMER_MONKEY_TEST_BUILDTAGS)
test-monkey-drummer:
	$(GOTEST) -o $(DRUMMER_MONKEY_TESTING_BIN) -c $(PKGNAME)
	./$(DRUMMER_MONKEY_TESTING_BIN) -test.v -test.timeout 9999s

slow-drummer: TESTTAGVALS+=$(DRUMMER_SLOW_TEST_BUILDTAGS)
slow-drummer:
	$(GOTEST) $(BUILD_TEST_ONLY) $(PKGNAME)

monkey-drummer: TESTTAGVALS+=$(DRUMMER_MONKEY_TEST_BUILDTAGS)
monkey-drummer:
	$(GOTEST) $(BUILD_TEST_ONLY) $(PKGNAME)

clean:
	rm -f $(DRUMMER_MONKEY_TESTING_BIN) $(PORCUPINE_CHECKER_BIN)

.PHONY: test clean $(PORCUPINE_CHECKER_BIN) $(DRUMMER_MONKEY_TESTING_BIN) \
	drummer-monkey-test-bin porcupine-checker
