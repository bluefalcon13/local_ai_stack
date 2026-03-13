#!/bin/bash

docker build \
    --build-arg BASE_IMAGE=ubuntu:26.04 \
    --build-arg VERSION=7.12.0a20260310 \
    --build-arg AMDGPU_FAMILY=gfx1151 \
    --build-arg RELEASE_TYPE=nightlies \
    -f rocm_runtime.Dockerfile \
    -t rocm:ubuntu26.04-gfx1151-7.12.0a20260310 \
    .