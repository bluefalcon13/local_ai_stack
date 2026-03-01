ARG UID=1001
ARG GID=1001
ARG USERNAME=ai-svc

# --- STAGE 1: BUILDER ---
FROM ubuntu:24.04 AS builder

# 1. We only need the bare minimum to run their script
RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates curl sudo wget gnupg2 \
    build-essential cmake ninja-build pkg-config \
    libdrm-dev libudev-dev libsystemd-dev libssl-dev \
    libncurses-dev opencl-headers ocl-icd-dev \
    libboost-all-dev libxml2-dev uuid-dev libprotoc-dev libjson-c-dev \
    python3-dev python3-pip python3-setuptools python3-pybind11 \
    flex bison rapidjson-dev systemtap-sdt-dev ocl-icd-opencl-dev \
    protobuf-compiler binutils libprotobuf-dev file \
    clang-18 llvm-18 lld-18 ninja-build libcurl4-openssl-dev \
    libavcodec-dev libavformat-dev libavutil-dev libswscale-dev libfftw3-dev && \
    rm -fr /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --recursive https://github.com/amd/xdna-driver.git .

# 2. Run the official AMD dependency script
# This will catch ncurses, OpenCL, Boost, and all the "hidden" requirements
RUN chmod +x ./tools/amdxdna_deps.sh && \
    ./tools/amdxdna_deps.sh  && \
    rm -fr /var/lib/apt/lists/*

# 3. Build XRT Base
WORKDIR /src/xrt/build
RUN export PROTOC_EXECUTABLE=$(which protoc) && \
    ./build.sh -npu -opt && \
    apt-get install -y ./Release/*.deb && \
    mkdir -p /to-install/xrt && cp ./Release/*.deb /to-install/xrt/ && \
    echo "/opt/xilinx/xrt/lib" > /etc/ld.so.conf.d/xrt.conf && ldconfig

ENV XILINX_XRT=/opt/xilinx/xrt
ENV PATH="${XILINX_XRT}/bin:${PATH}"

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

# 5. NOW BUILD FLM
# Sourcing xrt/setup.sh is the secret move that tells CMake where to look
WORKDIR /src
RUN git clone --recursive https://github.com/FastFlowLM/FastFlowLM.git && \
    cd ./FastFlowLM/src && mkdir -p /to-install/fastflowlm && \
    cmake --preset linux-default -G Ninja \
        -DCMAKE_INSTALL_PREFIX="/to-install/fastflowlm" \
        -DCMAKE_XCLBIN_PREFIX="/to-install/fastflowlm/share/flm" \
        -DCMAKE_CXX_FLAGS="-march=native" \
        -DXRT_DIR=/opt/xilinx/xrt/share/cmake/XRT \
        -DCMAKE_INCLUDE_PATH=/opt/xilinx/xrt/include/ \
        -DCMAKE_PREFIX_PATH=/opt/xilinx/xrt && \
    cmake --build build -j$(nproc) && \
    cmake --install build

# --- STAGE 2: THE APPLIANCE (Stage 3) ---
FROM ubuntu:24.04 AS final
ARG UID
ARG GID
ARG USERNAME

# install deps
RUN apt-get update && apt-get install -y \
    libcurl4 libfftw3-dev libavcodec60 libssl3 \
    libavformat60 libswscale7 && \
    rm -rf /var/lib/apt/lists/

# setup user account
RUN useradd -u $UID -m -s /bin/bash $USERNAME && \
    mkdir /home/$USERNAME/models && \
    chown -R $USERNAME:$USERNAME /home/$USERNAME

# transfer artifacts from previous stage
COPY --from=builder /to-install/fastflowlm/ /usr/local/
COPY --from=builder /to-install/xrt/ /tmp/
RUN apt update && apt install -y --no-install-recommends \
    /tmp/*.deb && \
    rm -fr /var/lib/apt/lists/*

RUN echo "/opt/xilinx/xrt/lib" > /etc/ld.so.conf.d/xrt.conf && ldconfig

# use user
USER $USERNAME
WORKDIR /home/$USERNAME

# set environment
ENV XILINX_XRT=/opt/xilinx/xrt
ENV PATH="${XILINX_XRT}/bin:${PATH}"

RUN mkdir /home/$USERNAME/config
