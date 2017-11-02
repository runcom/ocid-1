#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
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

# Dependencies:
# runc:
# - libseccomp-dev(Ubuntu,Debian)/libseccomp-devel(Fedora, CentOS, RHEL). Note that
# libseccomp in ubuntu <=trusty and debian <=jessie is not new enough, backport
# is required.
# - libapparmor-dev(Ubuntu,Debian)/libapparmor-devel(Fedora, CentOS, RHEL)

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"/..
. ${ROOT}/hack/versions

# DESTDIR is the dest path to install dependencies.
DESTDIR=${DESTDIR:-"/"}
# Convert to absolute path if it's relative.
if [[ ${DESTDIR} != /* ]]; then
  DESTDIR=${ROOT}/${DESTDIR}
fi

# NOSUDO indicates not to use sudo during installation.
NOSUDO=${NOSUDO:-false}
sudo="sudo"
if ${NOSUDO}; then
  sudo=""
fi

CRIO_DIR=${DESTDIR}/usr
RUNC_DIR=${DESTDIR}

RUNC_PKG=github.com/opencontainers/runc
CRITOOL_PKG=github.com/kubernetes-incubator/cri-tools
# Check GOPATH
if [[ -z "${GOPATH}" ]]; then
  echo "GOPATH is not set"
  exit 1
fi

# For multiple GOPATHs, keep the first one only
GOPATH=${GOPATH%%:*}

# checkout_repo checks out specified repository
# and switch to specified  version.
# Varset:
# 1) Repo name;
# 2) Version.
checkout_repo() {
  repo=$1
  version=$2
  path="${GOPATH}/src/${repo}"
  if [ ! -d ${path} ]; then
    mkdir -p ${path}
    git clone https://${repo} ${path}
  fi
  cd ${path}
  git fetch --all
  git checkout ${version}
}

# Install runc
checkout_repo ${RUNC_PKG} ${RUNC_VERSION}
cd ${GOPATH}/src/${RUNC_PKG}
BUILDTAGS=${BUILDTAGS:-seccomp selinux}
make BUILDTAGS="$BUILDTAGS"
${sudo} mkdir -p $DESTDIR/usr/bin
${sudo} cp runc $DESTDIR/usr/bin/runc

#Install crictl
checkout_repo ${CRITOOL_PKG} ${CRITOOL_VERSION}
cd ${GOPATH}/src/${CRITOOL_PKG}
make
