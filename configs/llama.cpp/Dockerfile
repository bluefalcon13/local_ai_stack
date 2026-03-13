ARG UID=1000
ARG GID=1000
ARG USERNAME=ai-svc
ARG VIDEO_GROUP=123
ARG RENDER_GROUP=456

FROM rocm:ubuntu26.04-gfx1151-7.12.0a20260310 AS base
ARG UID
ARG GID
ARG USERNAME
ARG VIDEO_GROUP
ARG RENDER_GROUP

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake git libatomic1 libgomp1 libssl-dev libssl3 ninja-build wget && \
    rm -fr /var/lib/apt/lists/*

# ROCM/llama Vars
ENV ROCM_PATH="/opt/rocm"
ENV PATH="${ROCM_PATH}/bin:${PATH}" \
    ROCM_HOME="${ROCM_PATH}" \
    HIP_PATH="${ROCM_PATH}" \
    HIP_PLATFORM="amd" \
    HIP_CLANG_PATH="${ROCM_PATH}/llvm/bin" \
    HIP_INCLUDE_PATH="${ROCM_PATH}/include" \
    CMAKE_PREFIX_PATH="${ROCM_PATH}/lib/cmake" \
    HIP_LIB_PATH="${ROCM_PATH}/lib" \
    HIP_DEVICE_LIB_PATH="${ROCM_PATH}/lib/llvm/amdgcn/bitcode" \
    AMDGPU_TARGETS="gfx1151" \
    GPU_TARGETS="gfx1151"
ENV PATH="${HIP_CLANG_PATH}/bin:${PATH}"

#create $USERNAME and groups with matching ids
RUN useradd -u $UID -m -s /bin/bash $USERNAME

RUN mkdir -p /home/$USERNAME/.cache/huggingface \
             /home/$USERNAME/models && \
    chown -R $USERNAME:$USERNAME /home/$USERNAME && \
    chmod -R 700 /home/$USERNAME

# add rocm libs to ld.so.conf.d so that ldconfig doesnt
# act like a blind and lost child...
RUN echo "/opt/rocm/lib" > /etc/ld.so.conf.d/rocm.conf && \
    echo "/opt/rocm/lib/llvm/lib" >> /etc/ld.so.conf.d/rocm.conf && \
    ldconfig

FROM base AS builder
WORKDIR /workdir

# build llama.cpp
RUN git clone https://github.com/ggml-org/llama.cpp.git && \
    cd ./llama.cpp/ && mkdir ./build && cd build && \
    cmake .. -G Ninja \
        -DCMAKE_C_COMPILER=${HIP_CLANG_PATH}/amdclang \
        -DCMAKE_CXX_COMPILER=${HIP_CLANG_PATH}/amdclang++ \
        -DCMAKE_C_FLAGS="-I/opt/rocm/include -march=znver5" \
        -DCMAKE_CXX_FLAGS="-I/opt/rocm/include -march=znver5" \
        -DCMAKE_INSTALL_RPATH="\$ORIGIN;/opt/rocm/lib" \
        -DCMAKE_BUILD_WITH_INSTALL_RPATH=ON \
        -DCMAKE_CROSSCOMPILING=OFF \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_SYSTEM_PROCESSOR=x86_64 \
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
        -DGGML_HIP_UMA=ON \
        -DLLAMA_CURL=OFF \
        -DGGML_NATIVE=ON \
        -DGGML_STATIC=OFF \
        -DCMAKE_SYSTEM_NAME=Linux && \
    cmake --build . -j$(nproc)

FROM base AS final
ARG USERNAME
ARG UID

USER $USERNAME
WORKDIR /home/$USERNAME

# copy in llama binarie
COPY --from=builder --chown=$UID:$UID --chmod=0754 /workdir/llama.cpp/build/bin/ /home/$USERNAME/llama/
ENV PATH="/home/$USERNAME/llama:$PATH"
ENV IMAGE="google/gemma-3-27b-it-qat-q4_0-gguf"
ENV XDG_CACHE_HOME=/home/$USERNAME/.cache/

CMD ["/bin/bash"]