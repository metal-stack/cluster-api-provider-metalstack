#!/usr/bin/env bash
set -eo pipefail

# this script has the following tasks:
# - install some CI test deps
# - cleanup remainings of prior test runs
# - provide a CI runner with a fresh version of mini-lab
#
# not intended to be run locally!

MINI_LAB_BRANCH=3-machines
MINI_LAB_REPO=https://github.com/metal-stack/mini-lab
MINI_LAB_PATH=./mini-lab

rm -rf "${MINI_LAB_PATH}"
git clone "${MINI_LAB_REPO}"

cd "${MINI_LAB_PATH}"

git checkout "${MINI_LAB_BRANCH}"

# self hosted runners get dirty, we need to clean up first
make cleanup
./test/ci-cleanup.sh

cd -