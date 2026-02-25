ARG UID=1000
ARG GID=1000
ARG USERNAME=ai-sv
ARG VIDEO_GROUP=123
ARG RENDER_GROUP=456

FROM ubuntu:24.04 AS base
ARG UID
ARG GID
ARG USERNAME
ARG VIDEO_GROUP
ARG RENDER_GROUP

# install deps for both runtime and building
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential ca-certificates cmake curl git gnupg libatomic1 \
    libcurl4 libgomp1 libssl-dev libssl3 ninja-build pkg-config \
    unzip wget zlib1g && \
    rm -fr /var/lib/apt/lists/*

# pull key from ubuntu
RUN mkdir -p --mode=0755 /etc/apt/keyrings && \
    wget -O - https://repo.amd.com/rocm/packages/gpg/rocm.gpg  | gpg --dearmor | tee /etc/apt/keyrings/amdrocm.gpg > /dev/null

# install rocm
RUN echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/amdrocm.gpg] https://repo.amd.com/rocm/packages/ubuntu2404 stable main" \
    | tee /etc/apt/sources.list.d/rocm.list && \
    apt-get update && apt-get install -y --no-install-recommends \
    amdrocm-core-sdk-gfx1151 && \
    rm -fr /var/lib/apt/lists/*

#create $USERNAME and groups with matching ids
RUN useradd -u $UID -m -s /bin/bash $USERNAME

USER $USERNAME
WORKDIR /home/$USERNAME

# ROCM/llama Vars
ENV ROCM_PATH=/opt/rocm/core
ENV ROCM_HOME="$ROCM_PATH" \
    HIP_PATH="$ROCM_PATH" \
    HIP_PLATFORM=amd \
    HIP_CLANG_PATH="$ROCM_PATH/llvm/bin" \
    HIP_INCLUDE_PATH="$ROCM_PATH/include" \
    CMAKE_PREFIX_PATH="$ROCM_PATH/lib/cmake" \
    HIP_DEVICE_LIB_PATH="$ROCM_PATH/lib/llvm/amdgcn/bitcode" \
    AMDGPU_TARGETS=gfx1151 \
    GPU_TARGETS=gfx1151
ENV PATH="$HIP_CLANG_PATH:$ROCM_PATH/bin:$PATH"

# Build the lemonade server
RUN git clone https://github.com/lemonade-sdk/lemonade.git && \
    cd ./lemonade && mkdir ./build && cd build && \
    cmake .. && cmake --build . --config Release -j"$(nproc)"

# copy in llama.cpp binary
COPY --chown=$UID:$GID --chmod=754 ./llama.cpp/build/bin/llama-server /home/$USERNAME/llama/bin/

ENV LEMONADE_LLAMACPP_ROCM_BIN=/home/$USERNAME/llama/bin/llama-server \
    LEMONADE_LLAMACPP_BACKEND=rocm \
    LEMONADE_LLAMACPP=rocm \
    LEMONADE_OFFLINE_MODE=1 \
    PATH="/home/$USERNAME/lemonade/build:/home/$USERNAME/llama/bin:$PATH"