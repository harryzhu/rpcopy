#!/bin/bash
PY3=$(which python3)
WORKDIR=$(cd $(dirname $0); pwd)

$PY3 $WORKDIR/jietu.py
