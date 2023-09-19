# Copyright 2017-2020 Lei Ni (nilei81@gmail.com)
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

GO := go
PKGNAME := github.com/lni/drummer/v3
MONKEY_TEST_TAG := dragonboat_monkeytest
MEMFS_TAG := dragonboat_memfs_test
PORCUPINE_CHECKER_BIN := porcupine-checker-bin
DRUMMER_MONKEY_TEST_BIN := drummer-monkey-testing
MONKEY_TEST_NAME := ^TestMonkeyPlay$$
CONCURRENT_TEST_NAME := ^TestConcurrentSMMonkeyPlay$$
ONDISK_TEST_NAME := ^TestOnDiskSMMonkeyPlay$$
JEPSEN_FILE := drummer-lcm.jepsen
EDN_FILE := drummer-lcm.edn
MAIN_PKG := github.com/lni/dragonboat/v4
COVER_PKG := $(MAIN_PKG),$(MAIN_PKG)/internal/raft,$(MAIN_PKG)/internal/rsm,$(MAIN_PKG)/internal/transport,$(MAIN_PKG)/internal/logdb,github.com/lni/dragonboat/v4/internal/logdb/kv/pebble,$(PKGNAME)

BUILD_TAGS := $(MONKEY_TEST_TAG)

.PHONY: all
all:
	@echo usage:
	@echo " make test"
	@echo " make test-tests"
	@echo " make monkey-test"
	@echo " make concurrent-monkey-test"
	@echo " make ondisk-monkey-test"
	@echo " make monkey-cover-test"
	@echo " make concurrent-monkey-cover-test"
	@echo " make ondisk-monkey-cover-test"
	@echo " make race-monkey-test"
	@echo " make race-concurrent-monkey-test"
	@echo " make race-ondisk-monkey-test"
	@echo " make memfs-monkey-test"
	@echo " make memfs-concurrent-monkey-test"
	@echo " make memfs-ondisk-monkey-test"
	@echo " make clean"
	@echo " "
	@echo "set the DRAGONBOAT_MEMFS_TEST environment varible to use memfs, e.g."
	@echo " DRAGONBOAT_MEMFS_TEST=1 make monkey-test"

.PHONY: $(DRUMMER_MONKEY_TEST_BIN)
$(DRUMMER_MONKEY_TEST_BIN):
	$(GO) test $(RACE) -tags="$(BUILD_TAGS)" -c -o $@ $(PKGNAME)

.PHONY: $(PORCUPINE_CHECKER_BIN)
$(PORCUPINE_CHECKER_BIN):
	$(GO) build -o $@ $(VERBOSE) $(PKGNAME)/lcm/checker

.PHONY: test
test: test-rsm
	$(GO) test -count=1 -v $(PKGNAME)

.PHONY: test-rsm
test-rsm:
	$(GO) test -count=1 -v -tags="$(MONKEY_TEST_TAG)" $(PKGNAME)/tests

.PHONY: runtest
runtest: $(PORCUPINE_CHECKER_BIN)
	$(GO) test -v -tags "$(BUILD_TAGS)" -timeout 3600s -run $(TARGET)
	if [ -f $(JEPSEN_FILE) ]; then \
    ./$(PORCUPINE_CHECKER_BIN) -path $(JEPSEN_FILE) -timeout 30; \
  fi

.PHONY: cover-test
cover-test:
	$(GO) test -v -tags $(MONKEY_TEST_TAG) -cover -coverprofile=coverage.out \
		-coverpkg $(COVER_PKG) -timeout 3600s -run $(TARGET)

.PHONY: cover-test-all
cover-test-all:
	$(GO) test -v -tags $(MONKEY_TEST_TAG) -cover -coverprofile=coverage.out \
    -coverpkg $(COVER_PKG) -timeout 7200s

.PHONY: monkey-test
monkey-test: override TARGET := $(MONKEY_TEST_NAME)
monkey-test: runtest

.PHONY: concurrent-monkey-test
concurrent-monkey-test: override TARGET := $(CONCURRENT_TEST_NAME)
concurrent-monkey-test: runtest

.PHONY: ondisk-monkey-test
ondisk-monkey-test: override TARGET := $(ONDISK_TEST_NAME)
ondisk-monkey-test: runtest

.PHONY: monkey-cover-test
monkey-cover-test: override TARGET := $(MONKEY_TEST_NAME)
monkey-cover-test: cover-test

.PHONY: concurrent-monkey-cover-test
concurrent-monkey-cover-test: override TARGET := $(CONCURRENT_TEST_NAME)
concurrent-monkey-cover-test: cover-test

.PHONY: ondisk-monkey-cover-test
ondisk-monkey-cover-test: override TARGET := $(ONDISK_TEST_NAME)
ondisk-monkey-cover-test: cover-test

.PHONY: race-monkey-test
race-monkey-test: override RACE := -race
race-monkey-test: monkey-test

.PHONY: race-concurrent-monkey-test
race-concurrent-monkey-test: override RACE := -race
race-concurrent-monkey-test: concurrent-monkey-test

.PHONY: race-ondisk-monkey-test
race-ondisk-monkey-test: override RACE := -race
race-ondisk-monkey-test: ondisk-monkey-test

.PHONY: memfs-monkey-test
memfs-monkey-test: BUILD_TAGS+=$(MEMFS_TAG)
memfs-monkey-test: monkey-test

.PHONY: memfs-concurrent-monkey-test
memfs-concurrent-monkey-test: BUILD_TAGS+=$(MEMFS_TAG)
memfs-concurrent-monkey-test: concurrent-monkey-test

.PHONY: memfs-ondisk-monkey-test
memfs-ondisk-monkey-test: BUILD_TAGS+=$(MEMFS_TAG)
memfs-ondisk-monkey-test: ondisk-monkey-test

.PHONY: memfs-monkey-test-bin
memfs-monkey-test-bin: BUILD_TAGS+=$(MEMFS_TAG)
memfs-monkey-test-bin: drummer-monkey-testing

.PHONY: clean
clean:
	@find . -type d -name "*safe_to_delete" -print | xargs rm -rf
	@rm -f $(PORCUPINE_CHECKER_BIN) $(JEPSEN_FILE) $(EDN_FILE) $(DRUMMER_MONKEY_TEST_BIN)
