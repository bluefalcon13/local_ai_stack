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
             /home/${USERNAME}/models && \
    chown -R ${USERNAME}:${USERNAME} /home/${USERNAME} && \
    chmod -R 700 /home/${USERNAME}

# --- 2. System Setup & Core Dependencies ---
RUN pacman-key --init && pacman -Syu --noconfirm && \
    # Install Chaotic-AUR for pre-built paru and ROCm packages
    pacman-key --recv-key 3056513887B78AEB --keyserver keyserver.ubuntu.com && \
    pacman-key --lsign-key 3056513887B78AEB && \
    pacman -U --noconfirm 'https://cdn-mirror.chaotic.cx/chaotic-aur/chaotic-keyring.pkg.tar.zst' \
    'https://cdn-mirror.chaotic.cx/chaotic-aur/chaotic-mirrorlist.pkg.tar.zst' && \
    echo -e "\n[chaotic-aur]\nInclude = /etc/pacman.d/chaotic-mirrorlist" >> /etc/pacman.conf && \
    pacman -Sy && \
    pacman -S --noconfirm --needed --ignore sudo \
        base-devel \
        blas \
        cmake \
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
        numactl \
        ocl-icd \
        openssl \
        paru \
        perl \
        python \
        python-pip \
        suitesparse \
        unzip \
        wget && \
    pacman -Scc --noconfirm

# --- 3. AUR Build (ROCm Binary) ---
RUN su $USERNAME -c "git clone https://aur.archlinux.org/rocm-gfx1151-bin.git /tmp/rocm-aur" && \
    cd /tmp/rocm-aur && \
    su $USERNAME -c "makepkg --noconfirm" && \
    pacman -U --noconfirm *.pkg.tar.zst && \
    cd / && rm -rf /tmp/rocm-aur

    # Finalize user home, permissions, and system library paths
RUN echo "/opt/rocm/lib" > /etc/ld.so.conf.d/rocm.conf && \
    echo "/opt/rocm/lib/llvm/lib" >> /etc/ld.so.conf.d/rocm.conf && \
    ldconfig

# --- 4. Environment Variables ---
ENV ROCM_PATH=/opt/rocm
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
    cmake -B build -S . \
        -DCMAKE_BUILD_TYPE=Release \
        -DLEMONADE_BUILD_WEBUI=ON \
        -DLEMONADE_BUILD_SERVER=ON && \
    cmake --build build --preset default web-ui -j$(nproc)

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
        -DLLAMA_CURL=OFF \
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
    ninja sd-cli sd-server

# --- 6. Final Stage ---
FROM base AS final
ARG USERNAME
ARG UID
ARG GID

USER ${USERNAME}
WORKDIR /home/${USERNAME}

# copy in llama binaries
COPY --from=builder --chown=${UID}:${GID} --chmod=0754 /workdir/llama.cpp/build/bin/ /home/${USERNAME}/llama/
# copy in Lemonade binaries
COPY --from=builder --chown=${UID}:${GID} --chmod=0754 /workdir/lemonade/build/ /home/${USERNAME}/lemonade/
# copy in SD binaries
COPY --from=builder --chown=${UID}:${GID} --chmod=0754 /workdir/stable-diffusion.cpp/build/bin/ /home/${USERNAME}/stable-diffusion/

ENV LEMONADE_LLAMACPP_ROCM_BIN=/home/${USERNAME}/llama/llama-server \
    LEMONADE_SDCPP_ROCM_BIN=/home/${USERNAME}/stable-diffusion/sd-server \
    LEMONADE_LLAMACPP_BACKEND=rocm \
    LEMONADE_LLAMACPP=rocm \
    LEMONADE_OFFLINE_MODE=1 \
    PATH="/home/${USERNAME}/lemonade:/home/${USERNAME}/llama:${PATH}"

# Default command: start server in headless mode
CMD ["lemonade-server", "serve", "--no-tray", "--host", "0.0.0.0", "--port", "42421"]
