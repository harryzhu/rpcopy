#!/bin/bash
WORKDIR=$(cd $(dirname $0); pwd)
cd $WORKDIR
coverage erase
coverage run ./jietu.py --model=debug
coverage run ./test_jietu.py
coverage xml coverage.xml
python-codacy-coverage -r coverage.xml