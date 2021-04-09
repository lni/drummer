#!/bin/bash

# Copyright 2017-2020 Lei Ni (nilei81@gmail.com)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

OS=`uname`
if [ $OS = "Darwin" ]; then
  # sudo diskutil erasevolume HFS+ "dragonboat-monkey-test" \
  #  `hdiutil attach -nomount ram://4629672`
  TARGETDIR="/Volumes/dragonboat-monkey-test"
else
  TARGETDIR="/media/dragonboat-monkey-test"
fi
EXECNAME="drummer-monkey-testing"
RUNTEST="run_monkey_test.sh"
PIDNAME="drummertest.pid"
MONKEYTOOL="monkey_tools.sh"
TARGETBIN=$TARGETDIR/bin

if [ ! -d "$TARGETDIR" ]; then
	echo "$TARGETDIR do not exist" >&2; exit 1
fi

if [ "$#" -ne 2 ]; then
  echo "usage: ./deploy.sh MODE NUM_OF_JOBS" >&2
  echo "MODE can be one of deploy, start or stop" >&2
  exit 1
fi

curdir=`pwd`
NUM_OF_JOBS=$2
MODE=$1
re='^[0-9]+$'
if ! [[ $NUM_OF_JOBS =~ $re ]] ; then
  echo "error: NUM_OF_JOBS parameter is not a number" >&2; exit 1
fi

seq=`seq 1 $NUM_OF_JOBS`

deploy()
{
  if [[ -z "${ONDISK_TEST}" ]]; then
    smtype="in memory"
    testsed="sed -e s/SELECTEDTEST/TestMonkeyPlay/g"
  else
    smtype="on disk"
    testsed="sed -e s/SELECTEDTEST/TestOnDiskSMMonkeyPlay/g"
  fi
  if [[ -z "${DRAGONBOAT_MEMFS_TEST}" ]]; then
    echo "regular test mode, sm type: $smtype"
    make -C .. drummer-monkey-testing
  else
    echo "memfs test mode, sm type: $smtype"
    make -C .. memfs-monkey-test-bin
  fi
  make -C .. porcupine-checker-bin
  rm -rf $TARGETDIR/*
  cp $MONKEYTOOL $TARGETDIR
  mkdir $TARGETBIN
  cp ../porcupine-checker-bin $TARGETBIN
  cp ../$EXECNAME $TARGETBIN
  mkdir $TARGETDIR/lcmlog
  base=24000
  incv=20
  for i in $seq
	do
    DIR="${TARGETDIR}/test${i}"
    mkdir -p $DIR
    mod=$(($i%10))
    if [ $mod -eq 0 ]; then
      cp dragonboat-drummer-no-snapshot.json $DIR/dragonboat-drummer.json
    elif [ $mod -eq 1 ]; then
      cp dragonboat-drummer-less-snapshot.json $DIR/dragonboat-drummer.json
    else
      cp dragonboat-drummer.json $DIR
    fi
    ln -s $TARGETDIR/bin/porcupine-checker-bin $DIR/porcupine-checker-bin
    ln -s $TARGETDIR/bin/$EXECNAME $DIR/$EXECNAME
    base=$((base+incv))
    sedcmd="sed -e s/BASEPORT/${base}/g"
    cat runscript_template.sh | ${testsed} | ${sedcmd} > $DIR/$RUNTEST
    chmod +x $DIR/$RUNTEST
  done
  return
}

starttests()
{
  for i in $seq
  do
    DIR="$TARGETDIR/test${i}"
    cd $DIR
    if [ ! -f $PIDNAME ]; then
      ./$RUNTEST & echo $! > $PIDNAME
    else
      echo "$PIDNAME already exist, skipping test in $DIR"
    fi
  done
  cd $curdir
  return
}

stoptests()
{
  for i in $seq
  do
    DIR="$TARGETDIR/test${i}"
    cd $DIR
    if [ -f $PIDNAME ]; then
      PID=`cat $PIDNAME`
      ps ax | grep $PID | grep $RUNTEST > /dev/null
      if [ $? -eq 0 ]; then
        kill -KILL $PID
      fi
      rm $PIDNAME
    else
      echo "drummertest.pid does not exist, skipping test in $DIR"
    fi
  done
  killall -KILL $EXECNAME
  cd $curdir
  return
}

if [[ "$MODE" == "deploy" ]]; then
  deploy
elif [[ "$MODE" == "start" ]]; then
  starttests
elif [[ "$MODE" == "stop" ]]; then
  stoptests
else
  echo "unsupported mode $MODE" >&2; exit 1
fi
