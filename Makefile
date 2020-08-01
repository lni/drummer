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
DRUMMER_MONKEY_TEST_TAGS := dragonboat_monkeytest
PORCUPINE_CHECKER_BIN := porcupine-checker-bin
MONKEY_TEST_NAME := TestMonkeyPlay
ONDISK_MONKEY_TEST_NAME := TestMonkeyPlayOnDiskSM 
JEPSEN_FILE := drummer-lcm.jepsen
EDN_FILE := drummer-lcm.edn

TEST_OPTION := -test.v -test.timeout 3200s

$(DRUMMER_MONKEY_TEST_BIN):
	$(GO) test $(RACE) -tags="$(DRUMMER_MONKEY_TEST_TAGS)" -c -o $@ $(PKGNAME)

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
	$(GO) test -count=1 -v -tags="$(DRUMMER_MONKEY_TEST_TAGS)" $(PKGNAME)/tests

.PHONY: runtest
runtest:
	./$(DRUMMER_MONKEY_TEST_BIN) $(TEST_OPTION) $(SILENT) -test.run $(TARGET)

.PHONY: monkey-test
monkey-test: TARGET := $(MONKEY_TEST_NAME)
monkey-test: $(DRUMMER_MONKEY_TEST_BIN)
monkey-test: runtest

.PHONY: ondisk-monkey-test
ondisk-monkey-test: TARGET := $(ONDISK_MONKEY_TEST_NAME)
ondisk-monkey-test: $(DRUMMER_MONKEY_TEST_BIN)
ondisk-monkey-test: runtest

.PHONY: monkey-test-travis
monkey-test-travis: override SILENT := -silent
monkey-test-travis: override TARGET := TestMonkeyPlayTravis
monkey-test-travis: $(DRUMMER_MONKEY_TEST_BIN)
monkey-test-travis: runtest

.PHONY: ondisk-monkey-test-travis
ondisk-monkey-test-travis: override SILENT := --silent
ondisk-monkey-test-travis: override TARGET := TestMonkeyPlayOnDiskSMTravis
ondisk-monkey-test-travis: $(DRUMMER_MONKEY_TEST_BIN)
ondisk-monkey-test-travis: runtest

.PHONY: race-monkey-test
race-monkey-test: RACE=-race
race-monkey-test: monkey-test

.PHONY: race-ondisk-monkey-test
race-ondisk-monkey-test: RACE=-race
race-ondisk-monkey-test: ondisk-monkey-test

.PHONY: clean
clean:
	@find . -type d -name "*safe_to_delete" -print | xargs rm -rf
	@rm -f $(DRUMMER_MONKEY_TEST_BIN) $(PORCUPINE_CHECKER_BIN)
	@rm -f $(JEPSEN_FILE) $(EDN_FILE) 
