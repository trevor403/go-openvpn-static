#!/usr/bin/env bash

CGO_LDFLAGS_ALLOW="-fobjc-arc" \
gomobile bind -target=ios/arm64 $@ -iosversion=10.3 -v github.com/trevor403/go-openvpn-static/openvpn3