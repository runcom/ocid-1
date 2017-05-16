FROM golang:1.7

# libseccomp in jessie is not _quite_ new enough -- need backports version
RUN echo 'deb http://httpredir.debian.org/debian jessie-backports main' > /etc/apt/sources.list.d/backports.list

#
# XXX: when adding dependencies here make sure to update the README.md as well
# as contrib/test/crio-integration-playbook.yaml.
#
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    gawk \
    iptables \
    pkg-config \
    libaio-dev \
    libcap-dev \
    libprotobuf-dev \
    libprotobuf-c0-dev \
    libseccomp2/jessie-backports \
    libseccomp-dev/jessie-backports \
    protobuf-c-compiler \
    protobuf-compiler \
    python-minimal \
    libglib2.0-dev \
    libjson-glib-dev \
    libapparmor-dev \
    btrfs-tools \
    libdevmapper1.02.1 \
    libdevmapper-dev \
    libgpgme11-dev \
    --no-install-recommends \
    && apt-get clean

# install bats
RUN cd /tmp \
    && git clone https://github.com/sstephenson/bats.git \
    && cd bats \
    && git reset --hard 03608115df2071fff4eaaff1605768c275e5f81f \
    && ./install.sh /usr/local

# install criu
ENV CRIU_VERSION 1.7
RUN mkdir -p /usr/src/criu \
    && curl -sSL https://github.com/xemul/criu/archive/v${CRIU_VERSION}.tar.gz | tar -v -C /usr/src/criu/ -xz --strip-components=1 \
    && cd /usr/src/criu \
    && make install-criu \
    && rm -rf /usr/src/criu

# Install runc
ENV RUNC_COMMIT v1.0.0-rc3
RUN set -x \
	&& export GOPATH="$(mktemp -d)" \
	&& git clone https://github.com/opencontainers/runc.git "$GOPATH/src/github.com/opencontainers/runc" \
	&& cd "$GOPATH/src/github.com/opencontainers/runc" \
	&& git fetch origin --tags \
	&& git checkout -q "$RUNC_COMMIT" \
	&& make static BUILDTAGS="seccomp selinux" \
	&& cp runc /usr/local/bin/runc \
	&& rm -rf "$GOPATH"

# Install CNI plugins
ENV CNI_COMMIT d4bbce1865270cd2d2be558d6a23e63d314fe769
RUN set -x \
       && export GOPATH="$(mktemp -d)" \
       && git clone https://github.com/containernetworking/cni.git "$GOPATH/src/github.com/containernetworking/cni" \
       && cd "$GOPATH/src/github.com/containernetworking/cni" \
       && git checkout -q "$CNI_COMMIT" \
       && ./build.sh \
       && mkdir -p /opt/cni/bin \
       && cp bin/* /opt/cni/bin/ \
       && rm -rf "$GOPATH"

COPY test/plugin_test_args.bash /opt/cni/bin/plugin_test_args.bash

# Make sure we have some policy for pulling images
RUN mkdir -p /etc/containers
COPY test/policy.json /etc/containers/policy.json

WORKDIR /go/src/github.com/kubernetes-incubator/cri-o

ADD . /go/src/github.com/kubernetes-incubator/cri-o

RUN make copyimg \
	&& mkdir -p .artifacts/redis-image \
	&& ./test/copyimg/copyimg --import-from=docker://redis --export-to=dir:.artifacts/redis-image --signature-policy ./test/policy.json
