#!/bin/bash
WORKDIR=$(cd $(dirname $0); pwd)
cd $WORKDIR
coverage xml
python-codacy-coverage -r coverage.xml