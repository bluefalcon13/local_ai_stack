FROM ubuntu:24.04 AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake ninja-build git \
    ca-certificates gnupg wget libatomic1 \
    libssl-dev && \
    rm -fr /var/lib/apt/lists/*

# pull key from ubuntu
RUN mkdir -p --mode=0755 /etc/apt/keyrings && \
    wget -O - https://repo.amd.com/rocm/packages/gpg/rocm.gpg  | gpg --dearmor | tee /etc/apt/keyrings/amdrocm.gpg > /dev/null

RUN echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/amdrocm.gpg] https://repo.amd.com/rocm/packages/ubuntu2404 stable main" \
    | tee /etc/apt/sources.list.d/rocm.list && \
    apt-get update && apt-get install -y --no-install-recommends \
    amdrocm-core-sdk-gfx1151 && \
    rm -fr /var/lib/apt/lists/*

ENV ROCM_PATH=/opt/rocm/core
ENV ROCM_HOME="$ROCM_PATH" \
    HIP_PATH="$ROCM_PATH" \
    HIP_PLATFORM=amd \
    HIP_CLANG_PATH="$ROCM_PATH/llvm/bin" \
    HIP_INCLUDE_PATH="$ROCM_PATH/include" \
    CMAKE_PREFIX_PATH="$ROCM_PATH/lib/cmake" \
    HIP_DEVICE_LIB_PATH="$ROCM_PATH/lib/llvm/amdgcn/bitcode" \
    AMDGPU_TARGETS=gfx1151
ENV PATH="$HIP_CLANG_PATH:$ROCM_PATH/bin:$PATH"

WORKDIR /app
RUN git clone https://github.com/lemonade-sdk/lemonade.git && \
    cd ./lemonade && mkdir ./build && \
    cmake -S . -B build && cmake --build build && \
    cmake --install build -j$(nproc)
    # && cd .. && rm -fr ./lemonade