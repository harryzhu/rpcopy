#!/bin/bash
PY3=`which python3`
WORKDIR=$(cd $(dirname $0); pwd)
cd $WORKDIR
coverage xml
python-codacy-coverage -r coverage.xml

$PY3 $WORKDIR/jietu.py
