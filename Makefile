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
DRUMMER_MONKEY_TEST_BIN := drummer-monkey-testing
DRUMMER_MONKEY_TEST_TAG := dragonboat_monkeytest
MEMFS_TAG := dragonboat_memfs_test
PORCUPINE_CHECKER_BIN := porcupine-checker-bin
MONKEY_TEST_NAME := TestMonkeyPlay$$
ONDISK_MONKEY_TEST_NAME := TestOnDiskSMMonkeyPlay$$
TRAVIS_ONDISK_MONKEY_TEST_NAME := TestOnDiskSMMonkeyPlayTravis$$
TRAVIS_MONKEY_TEST_NAME := TestMonkeyPlayTravis$$
JEPSEN_FILE := drummer-lcm.jepsen
EDN_FILE := drummer-lcm.edn

TEST_OPTION := -test.v -test.timeout 3200s

BUILD_TAGS := $(DRUMMER_MONKEY_TEST_TAG)
ifneq ($(DRAGONBOAT_MEMFS_TEST),)
$(info using memfs based pebble)
BUILD_TAGS+=$(MEMFS_TAG)
endif

.PHONY: all
all:
	@echo usage:
	@echo " make test"
	@echo " make test-tests"
	@echo " make monkey-test"
	@echo " make ondisk-monkey-test"
	@echo " make monkey-test-travis"
	@echo " make ondisk-monkey-test-travis"
	@echo " make race-monkey-test"
	@echo " make race-ondisk-monkey-test"
	@echo " make race-monkey-test-travis"
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

.PHONY: check
check:
	if [ -f $(JEPSEN_FILE) ]; then \
  	./$(PORCUPINE_CHECKER_BIN) -path $(JEPSEN_FILE) -timeout 30; \
 	fi

.PHONY: test
test:
	$(GO) test -count=1 -v $(PKGNAME)

.PHONY: test-tests
test-tests:
	$(GO) test -count=1 -v -tags="$(DRUMMER_MONKEY_TEST_TAG)" $(PKGNAME)/tests

.PHONY: runtest
runtest:
	./$(DRUMMER_MONKEY_TEST_BIN) $(TEST_OPTION) $(SILENT) $(SLOWVM) -test.run $(TARGET)

.PHONY: monkey-test
monkey-test: override TARGET := $(MONKEY_TEST_NAME)
monkey-test: $(DRUMMER_MONKEY_TEST_BIN)
monkey-test: runtest

.PHONY: ondisk-monkey-test
ondisk-monkey-test: override TARGET := $(ONDISK_MONKEY_TEST_NAME)
ondisk-monkey-test: $(DRUMMER_MONKEY_TEST_BIN)
ondisk-monkey-test: runtest

.PHONY: monkey-test-travis
monkey-test-travis: override SILENT := -silent
monkey-test-travis: override SLOWVM := -slowvm
monkey-test-travis: override TARGET := $(TRAVIS_MONKEY_TEST_NAME)
monkey-test-travis: $(DRUMMER_MONKEY_TEST_BIN)
monkey-test-travis: runtest

.PHONY: ondisk-monkey-test-travis
ondisk-monkey-test-travis: override SILENT := --silent
ondisk-monkey-test-travis: override SLOWVM := -slowvm
ondisk-monkey-test-travis: override TARGET := $(TRAVIS_ONDISK_MONKEY_TEST_NAME)
ondisk-monkey-test-travis: $(DRUMMER_MONKEY_TEST_BIN)
ondisk-monkey-test-travis: runtest

.PHONY: race-monkey-test
race-monkey-test: override RACE := -race
race-monkey-test: monkey-test

.PHONY: race-ondisk-monkey-test
race-ondisk-monkey-test: override RACE := -race
race-ondisk-monkey-test: ondisk-monkey-test

.PHONY: race-monkey-test-travis
race-monkey-test-travis: override RACE := -race
race-monkey-test-travis: override SLOWVM := -slowvm
race-monkey-test-travis: monkey-test-travis

.PHONY: clean
clean:
	@find . -type d -name "*safe_to_delete" -print | xargs rm -rf
	@rm -f $(DRUMMER_MONKEY_TEST_BIN) $(PORCUPINE_CHECKER_BIN)
	@rm -f $(JEPSEN_FILE) $(EDN_FILE) 
