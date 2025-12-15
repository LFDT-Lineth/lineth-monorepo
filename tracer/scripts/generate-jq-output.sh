#!/bin/bash

# first arg is module name eg add
module=$1

# make output dir
OUTDIR=jq-$module
mkdir $OUTDIR

# run jq on each file, and the output goes into that dir
for f in *.json
do
  echo "jq .$module $f > $OUTDIR/$f"
  jq .$module $f > $OUTDIR/$f
done
