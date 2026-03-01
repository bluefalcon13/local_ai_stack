#!/bin/bash

docker build \
    --build-arg BASE_IMAGE=ubuntu:24.04 \
    --build-arg VERSION=7.11.0 \
    --build-arg AMDGPU_FAMILY=gfx1151 \
    --build-arg RELEASE_TYPE=stable \
    -f rocm_runtime.Dockerfile \
    -t rocm:ubuntu24.04-gfx1151-7.11.0 \
    .