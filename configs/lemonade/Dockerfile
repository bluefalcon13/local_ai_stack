ARG UID=1000
ARG GID=1000
ARG USERNAME=ai-svc
ARG VIDEO_GROUP=123
ARG RENDER_GROUP=456

FROM archlinux:latest AS base
ARG UID
ARG GID
ARG USERNAME

# --- 1. Create User and Home Directory setup for final
# stage. User can be utilized for paru installs
RUN groupadd -g ${GID} ${USERNAME} || true && \
    useradd -u ${UID} -g ${GID} -m -s /bin/bash ${USERNAME} && \
    mkdir -p /home/${USERNAME}/.cache/lemonade \
             /home/${USERNAME}/.cache/huggingface \
             /home/${USERNAME}/models \
             /home/${USERNAME}/.config/flm/models && \
    chown -R ${USERNAME}:${USERNAME} /home/${USERNAME} && \
    chmod -R 700 /home/${USERNAME}

# --- 2. System Setup & Core Dependencies ---
RUN pacman-key --init && \
    sed -i '/^#\[core-testing\]/,/^#Include/ s/^#//' /etc/pacman.conf && \
    sed -i '/^#\[extra-testing\]/,/^#Include/ s/^#//' /etc/pacman.conf && \
    pacman -Syu --noconfirm && \
    pacman -S --noconfirm --needed --assume-installed sudo \
        base-devel \
        blas \
        cmake \
        curl \
        elfutils \
        file \
        git \
        kmod \
        lapack \
        libdrm \
        libglvnd \
        libunwind \
        ncurses \
        ninja \
        npm \
        nodejs \
        numactl \
        ocl-icd \
        openssl \
        perl \
        python \
        python-pip \
        python-pyelftools \
        suitesparse \
        unzip \
        wget \
        which \
        xrt \
        xrt-plugin-amdxdna && \
    pacman -Scc --noconfirm

# --- 3. AUR Build (ROCm Binary) ---
COPY ./sideload/rocm-gfx1151-bin-7.11.0pre-1-x86_64.pkg.tar.zst /tmp/rocm/
RUN --mount=type=bind,src=./sideload/,dst=/tmp/sideload/ \
    pacman -U /tmp/sideload/rocm-gfx1151-bin-7.11.0pre-1-x86_64.pkg.tar.zst --noconfirm && \
    pacman -Scc --noconfirm

    # Finalize user home, permissions, and system library paths
RUN echo "/opt/rocm/lib" > /etc/ld.so.conf.d/rocm.conf && \
    echo "/opt/rocm/lib/llvm/lib" >> /etc/ld.so.conf.d/rocm.conf && \
    echo "/usr/lib" > /etc/ld.so.conf.d/xrt.conf && \
    ldconfig

# --- 4. Environment Variables ---
ENV ROCM_PATH=/opt/rocm \
    XILINX_XRT=/usr
ENV PATH=${ROCM_PATH}/bin:${ROCM_PATH}/llvm/bin:$PATH \
    ROCM_HOME=${ROCM_PATH} \
    HIP_PATH=${ROCM_PATH} \
    HIP_PLATFORM=amd \
    HIP_CLANG_PATH=${ROCM_PATH}/llvm/bin \
    HIP_INCLUDE_PATH=${ROCM_PATH}/include \
    CMAKE_PREFIX_PATH=${ROCM_PATH}/lib/cmake \
    HIP_DEVICE_LIB_PATH=${ROCM_PATH}/lib/llvm/amdgcn/bitcode \
    AMDGPU_TARGETS=gfx1151 \
    GPU_TARGETS=gfx1151
ENV PATH="$HIP_CLANG_PATH/bin:$PATH"

# --- 5. Builder Stage ---
FROM base AS builder
WORKDIR /workdir

# build lemonade
RUN git clone https://github.com/lemonade-sdk/lemonade.git && \
    cd ./lemonade && \
    cmake -G Ninja -B build -S . \
        -DCMAKE_BUILD_TYPE=Release \
        -DLEMONADE_ENABLE_RYZENAI=ON \
        -DCMAKE_C_FLAGS="-march=native -O3 -pipe" \
        -DCMAKE_CXX_FLAGS="-march=native -O3 -pipe" && \
    cmake --build build \
        --target lemonade-server \
        --target lemonade-router \
        --target web-app \
        -j$(nproc)

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
        -DCMAKE_C_COMPILER=/opt/rocm/llvm/bin/amdclang \
        -DCMAKE_CXX_COMPILER=/opt/rocm/llvm/bin/amdclang++ \
        -DCMAKE_CXX_FLAGS="-I/opt/rocm/include" \
        -DCMAKE_INSTALL_RPATH="\$ORIGIN;/opt/rocm/lib" \
        -DCMAKE_BUILD_WITH_INSTALL_RPATH=ON \
        -DCMAKE_CROSSCOMPILING=OFF \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_HIP_FLAGS="-mllvm --amdgpu-unroll-threshold-local=600" \
        -DGPU_TARGETS="gfx1151" \
        -DBUILD_SHARED_LIBS=OFF \
        -DLLAMA_BUILD_TESTS=OFF \
        -DGGML_AVX512=ON \
        -DGGML_AVX512_VNNI=ON \
        -DGGML_HIPBLASLT=ON \
        -DGGML_HIP=ON \
        -DGGML_OPENMP=ON \
        -DGGML_CUDA_FORCE_CUBLAS=OFF \
        -DGGML_HIP_ROCWMMA_FATTN=ON \
        -DGGML_CUDA_FA_ALL_QUANTS=ON \
        -DGGML_CUDA_F16_256=ON \
        -DLLAMA_HIP_UMA=ON \
        -DLLAMA_CURL=ON \
        -DGGML_NATIVE=ON \
        -DGGML_STATIC=OFF \
        -DCMAKE_SYSTEM_NAME=Linux && \
    cmake --build . -j$(nproc)

# build SD.cpp
RUN git clone --recursive https://github.com/lemonade-sdk/stable-diffusion.cpp.git && \
    cd ./stable-diffusion.cpp/ && mkdir build && cd ./build && \
    cmake .. -G Ninja \
        -DCMAKE_C_COMPILER=/opt/rocm/llvm/bin/amdclang \
        -DCMAKE_CXX_COMPILER=/opt/rocm/llvm/bin/amdclang++ \
        -DSD_HIPBLAS=ON \
        -DAMDGPU_TARGETS="gfx1151" \
        -DSD_BUILD_EXAMPLES=ON \
        -DGGML_FLASH_ATTN=ON \
        -DGGML_AVX512=ON \
        -DGGML_AVX512_VNNI=ON \
        -DGGML_HIPBLASLT=ON \
        -DGGML_NATIVE=ON \
        -DCMAKE_BUILD_TYPE=Release && \
    ninja sd-cli sd-server -j$(nproc)

# --- 6. Final Stage ---
FROM base AS final
ARG USERNAME
ARG UID
ARG GID

# copy in llama binaries
COPY --from=builder --chown=${UID}:${GID} --chmod=0754 /workdir/llama.cpp/build/bin/ /home/${USERNAME}/llama/
# copy in Lemonade binaries and web-app
COPY --from=builder --chown=${UID}:${GID} --chmod=0754 \
    /workdir/lemonade/build/lemonade-router \
    /workdir/lemonade/build/lemonade-server \
    /home/${USERNAME}/lemonade/

COPY --from=builder --chown=${UID}:${GID} --chmod=0754 \
    /workdir/lemonade/build/resources/ \
    /home/${USERNAME}/lemonade/resources/
# copy in SD binaries
COPY --from=builder --chown=${UID}:${GID} --chmod=0754 \
    /workdir/stable-diffusion.cpp/build/bin/ \
    /home/${USERNAME}/stable-diffusion/

RUN pacman -S --noconfirm base-devel boost git cmake ninja rust  ffmpeg && \
    git clone --recursive https://github.com/FastFlowLM/FastFlowLM.git /tmp/FastFlowLM && \
    cd /tmp/FastFlowLM/src && \
    cmake --preset linux-default -G Ninja \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_CXX_FLAGS="-march=native -O3 -pipe" \
        -DXRT_DIR="/usr" && \
    cmake --build build -j$(nproc) && \
    cmake --install build && \
    cd / && \
    rm -rf /tmp/FastFlowLM && \
    pacman -Rs --noconfirm base-devel git cmake ninja rust && \
    pacman -Scc --noconfirm

USER ${USERNAME}
WORKDIR /home/${USERNAME}

ENV LEMONADE_LLAMACPP_ROCM_BIN=/home/${USERNAME}/llama/llama-server \
    LEMONADE_SDCPP_ROCM_BIN=/home/${USERNAME}/stable-diffusion/sd-server \
    LEMONADE_LLAMACPP_BACKEND=rocm \
    LEMONADE_LLAMACPP=rocm \
    LEMONADE_OFFLINE_MODE=1 \
    ROCBLAS_USE_HIPBLASLT=1 \
    LEMONADE_USE_NPU=1 \
    GGML_NUMA=1 \
    PATH="/home/${USERNAME}/lemonade:/home/${USERNAME}/llama:${PATH}"

# Default command: start server in headless mode
CMD ["lemonade-server", "serve", "--no-tray", "--host", "0.0.0.0", "--port", "42421"]
