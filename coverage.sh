#!/bin/bash
WORKDIR=$(cd $(dirname $0); pwd)
cd $WORKDIR
#coverage erase
coverage run ./jietu.py --model=debug
sleep 1
coverage run ./test_jietu.py
sleep 1
coverage xml #coverage.xml
sleep 1
python-codacy-coverage -r coverage.xml