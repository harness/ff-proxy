#!/bin/bash
# Copyright 2025 Harness Inc. All rights reserved.
# Use of this source code is governed by the PolyForm Free Trial 1.0.0 license
# that can be found in the licenses directory at the root of this repository, also available at
# https://polyformproject.org/wp-content/uploads/2020/05/PolyForm-Free-Trial-1.0.0.txt.

###################################################
# Purpose
# The purpose of this script is to facilitate auto-tagging of
# Jira tickets with fix-versions. There are two tricky parts to auto-tagging:
#
# 1. Given a change set (PR diff), what constitutes a change to a service?
# 2. Given you've determined a set of files has changed a service, which service was changed?
#
# This script endeavors to answer question number one - which files constitute a change to a service
# Given an input file which is the git diff from a PR this script should determine what file changes in the diff
# constitute a material change to a service. At the time of this writing, this currently only identifies java and go
# files, and if those files are in the change list, then those file names are returned in the output file.
#
# Inputs
# $1 - the git diff from the pull request
# $2 - the output file which should ultimately contain file names from the diff that affect a service
#
# Reference Jira Tickets
# See BT-10437
# See BT-11220
# See BT-11235
# See BT-11274
# See BT-11288
#
# Owner: Engops
# Author: Marc Batchelor
###################################################
uniqueFileNamesFile="$1"
sourceDiffNames="$2"

# These folders will be ignored as "source code"
excludePathRegex="^scripts/|/test/|^.engops/|^.harness/|^.vscode/|^docs/|^license/|^tests/|^.run"

# Every file extension here is considered "source code"
codeFileExtensionsRegex="Dockerfile|Dockerfile.cov|Dockerfile.dev|bazel|c|cc|conf|css|ejs|eslintrc|gitmodules|golang|gradle|gv|graphql|h|html|iml|js|json|less|mod|pipeline|mustache|pl|png|properties|ps1|proto|py|pyc|qbg|repo|rs|sh|sha256|sql|sum|svg|tf|tgz|tmpl|tpl|ts|tsx|xml|yaml|yml"

# These are file names, not extensions
codeFileNamesRegex="Dockerfile|Dockerfile.multistage"

if [ -z "$uniqueFileNamesFile" ]; then
  echo "Missing input PR Difference file."
  exit 1
fi
if [ ! -f "$uniqueFileNamesFile" ]; then
  echo "Input file $uniqueFileNamesFile does not exist and is required."
  exit 2
fi
if [ -z "$sourceDiffNames" ]; then
  echo "Missing output file."
  exit 3
fi
if [ ! -f "$sourceDiffNames" ]; then
  echo "File $sourceDiffNames does not exist and is required."
  exit 4
fi

##### Detect git diff file, or processed filenames only
isDiffFile=$(grep -E "^diff --git a\/" "$uniqueFileNamesFile" | wc -l)
if [ $isDiffFile -gt 0 ]; then
  echo "Received a diff file... fix it to be a filenames only file"
  fileNamesOnlyVar=$(cat "$uniqueFileNamesFile"|grep -E "^diff --git" | sed 's/diff --git a\///' | sed 's/ b\/.*$//' | sort -u)
  echo -e "$fileNamesOnlyVar">"$uniqueFileNamesFile"
fi

# Java files (and other files) which end up in jars - these are kept in .../src/main/x/x/x/* 
cat "$uniqueFileNamesFile" | grep -E ".*.java$" | grep -v "/test/" > $sourceDiffNames
# go files (without tests)
cat "$uniqueFileNamesFile" | grep -E ".*.(go|mod)$" | grep -v "test_" >> $sourceDiffNames
# Other source files with specific extensions
cat "$uniqueFileNamesFile" | grep -E ".*.($codeFileExtensionsRegex)$" | grep -v -E "$excludePathRegex" >> $sourceDiffNames
# Actual file names
cat "$uniqueFileNamesFile" | grep -E "($codeFileNamesRegex)$" | grep -v -E "$excludePathRegex" >> $sourceDiffNames
