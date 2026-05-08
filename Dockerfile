# Copyright (C) 2023-2026  Red Hat, Inc.
#
# This copyrighted material is made available to anyone wishing to use,
# modify, copy, or redistribute it subject to the terms and conditions of
# the GNU General Public License v.2, or (at your option) any later version.
# This program is distributed in the hope that it will be useful, but WITHOUT
# ANY WARRANTY expressed or implied, including the implied warranties of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
# Public License for more details.  You should have received a copy of the
# GNU General Public License along with this program; if not, write to the
# Free Software Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
# 02110-1301, USA.  Any Red Hat trademarks that are incorporated in the
# source code or documentation are not subject to the GNU General Public
# License and may only be used or replicated with the express permission of
# Red Hat, Inc.
#
# Red Hat Author(s): Todd Gill <tgill@redhat.com>
#

# Build stage
FROM golang:1.22-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build the driver
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -ldflags '-extldflags "-static"' \
    -o csi-driver ./cmd/csi-driver

# Runtime stage
FROM alpine:3.19

ARG BUILD_DATE

LABEL maintainers="Todd Gill <tgill@redhat.com>" \
    version="0.3.0" \
    release="1" \
    description="Springfield CSI Driver" \
    summary="Springfield CSI driver for local storage" \
    org.label-schema.schema-version="1.0" \
    org.label-schema.name="springfield-csi" \
    org.label-schema.description="Springfield CSI Driver" \
    org.label-schema.url="https://github.com/springfield-csi/driver" \
    org.label-schema.build-date=${BUILD_DATE} \
    org.opencontainers.image.source="https://github.com/springfield-csi/driver" \
    org.opencontainers.image.description="Springfield CSI driver for local storage"

# Install storage management tools
RUN apk add --no-cache \
    ca-certificates \
    util-linux \
    e2fsprogs \
    e2fsprogs-extra \
    xfsprogs \
    xfsprogs-extra \
    btrfs-progs \
    lvm2 \
    lvm2-extra \
    mdadm \
    device-mapper \
    device-mapper-libs \
    cryptsetup \
    blkid \
    findmnt

# Note: Stratis is not available in Alpine repositories
# For Stratis support, consider using Fedora/RHEL base image

# Create necessary directories
RUN mkdir -p /csi /var/lib/springfield-csi

# Copy the binary from builder
COPY --from=builder /workspace/csi-driver /usr/local/bin/

# Set permissions
RUN chmod +x /usr/local/bin/csi-driver

ENTRYPOINT ["/usr/local/bin/csi-driver"]
