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

RUN --mount=type=bind,source=./install_rocm_deps.sh,target=/install_rocm_deps.sh \
    /install_rocm_deps.sh

# install deps for both runtime and building
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake curl git gnupg libatomic1 \
    libcurl4 libgomp1 libssl-dev libssl3 ninja-build pkg-config \
    unzip wget zlib1g && \
    rm -fr /var/lib/apt/lists/*

RUN --mount=type=bind,source=./install_rocm_tarball.sh,target=/install_rocm_tarball.sh \
    /install_rocm_tarball.sh 7.11.0 gfx1151 stable

# ROCM/llama Vars
ENV ROCM_PATH=/opt/rocm
ENV PATH=${ROCM_PATH}/bin:$PATH \
    ROCM_HOME="$ROCM_PATH" \
    HIP_PATH="$ROCM_PATH" \
    HIP_PLATFORM=amd \
    HIP_CLANG_PATH="$ROCM_PATH/llvm/bin" \
    HIP_INCLUDE_PATH="$ROCM_PATH/include" \
    CMAKE_PREFIX_PATH="$ROCM_PATH/lib/cmake" \
    HIP_DEVICE_LIB_PATH="$ROCM_PATH/lib/llvm/amdgcn/bitcode" \
    AMDGPU_TARGETS=gfx1151 \
    GPU_TARGETS=gfx1151
ENV PATH="$HIP_CLANG_PATH/bin:$PATH"

#create $USERNAME and groups with matching ids
RUN useradd -u $UID -m -s /bin/bash $USERNAME

RUN mkdir -p /home/$USERNAME/.cache/lemonade \
             /home/$USERNAME/.cache/huggingface \
             /home/$USERNAME/models && \
    chown -R $USERNAME:$USERNAME /home/$USERNAME && \
    chmod -R 700 /home/$USERNAME

FROM base AS builder
WORKDIR /workdir
# build lemonade
RUN git clone https://github.com/lemonade-sdk/lemonade.git && \
    cd ./lemonade && \
    cmake -B build -S . \
        -DCMAKE_BUILD_TYPE=Release \
        -DLEMONADE_BUILD_WEBUI=ON \
        -DLEMONADE_BUILD_SERVER=ON && \
    cmake --build build -j$(nproc)

# build llama.cpp
RUN git clone https://github.com/ggml-org/llama.cpp.git && \
    cd ./llama.cpp/ && mkdir ./build && cd build && \
    export HIP_PATH=/opt/rocm && \
    export ROCM_PATH=/opt/rocm && \
    export HIP_PLATFORM=amd && \
    export HIP_CLANG_PATH=/opt/rocm/llvm/bin && \
    export HIP_INCLUDE_PATH=/opt/rocm/include && \
    export HIP_LIB_PATH=/opt/rocm/lib && \
    export HIP_DEVICE_LIB_PATH=/opt/rocm/lib/llvm/amdgcn/bitcode && \
    export PATH=/opt/rocm/bin:/opt/rocm/llvm/bin:$PATH && \
    export LD_LIBRARY_PATH=/opt/rocm/lib:/opt/rocm/lib64:/opt/rocm/llvm/lib:${LD_LIBRARY_PATH:-} && \
    export LIBRARY_PATH=/opt/rocm/lib:/opt/rocm/lib64:${LIBRARY_PATH:-} && \
    export CPATH=/opt/rocm/include:${CPATH:-} && \
    export PKG_CONFIG_PATH=/opt/rocm/lib/pkgconfig:${PKG_CONFIG_PATH:-} && \
    cmake .. -G Ninja \
        -DCMAKE_C_COMPILER=/opt/rocm/llvm/bin/clang \
        -DCMAKE_CXX_COMPILER=/opt/rocm/llvm/bin/clang++ \
        -DCMAKE_CXX_FLAGS="-I/opt/rocm/include" \
        -DCMAKE_CROSSCOMPILING=OFF \
        -DCMAKE_BUILD_TYPE=Release \
        -DGPU_TARGETS="gfx1151" \
        -DBUILD_SHARED_LIBS=OFF \
        -DLLAMA_BUILD_TESTS=OFF \
        -DGGML_HIP=ON \
        -DGGML_OPENMP=ON \
        -DGGML_CUDA_FORCE_CUBLAS=OFF \
        -DGGML_HIP_ROCWMMA_FATTN=ON \
        -DLLAMA_CURL=OFF \
        -DGGML_NATIVE=ON \
        -DGGML_STATIC=OFF \
        -DCMAKE_SYSTEM_NAME=Linux && \
    cmake --build . -j$(nproc) && cd ./bin && \
    cp -v /opt/rocm/lib/*.so* . && \
    # cp -v /opt/rocm/lib64/*.so* . && \
    cp -v /opt/rocm/lib/llvm/lib/*.so* . && \
    cp -v /opt/rocm/lib/rocm_sysdeps/lib/*.so* . && \
    mkdir -p rocblas && cp -r /opt/rocm/lib/rocblas/library rocblas/ && \
    mkdir -p hipblaslt && cp -r /opt/rocm/lib/hipblaslt/library hipblaslt/

FROM base AS final
ARG USERNAME
ARG UID

USER $USERNAME
WORKDIR /home/$USERNAME

# copy in llama binarie
COPY --from=builder --chown=$UID:$UID --chmod=0754 /workdir/llama.cpp/build/bin/ /home/$USERNAME/llama/
# copy in Lemonade binaries
COPY --from=builder --chown=$UID:$GID --chmod=754 /workdir/lemonade/build/ /home/$USERNAME/lemonade/
# COPY --from=builder --chown=$UID:$GID --chmod=754 /workdir/lemonade/build/lemonade-router /home/$USERNAME/lemonade/lemonade-router



ENV LEMONADE_LLAMACPP_ROCM_BIN=/home/$USERNAME/llama/llama-server \
    LEMONADE_LLAMACPP_BACKEND=rocm \
    LEMONADE_LLAMACPP=rocm \
    LEMONADE_OFFLINE_MODE=1 \
    PATH="/home/$USERNAME/lemonade:/home/$USERNAME/llama:$PATH"

# Default command: start server in headless mode
CMD ["lemonade-server", "serve", "--no-tray", "--host", "0.0.0.0", "--port", "42421"]