FROM archlinux:latest

ARG UID=1001
ARG GID=1001
ARG USERNAME=ai-svc

# 1. Enable extra-testing for XDNA and setup persistent runtime deps
RUN echo -e "[extra-testing]\nInclude = /etc/pacman.d/mirrorlist\n" >> /etc/pacman.conf && \
    pacman -Syu --noconfirm && \
    pacman -S --noconfirm \
    xrt xrt-plugin-amdxdna boost-libs ocl-icd fftw ffmpeg libdrm openssl 

# 2. Setup user early so we can chown the install dir
RUN groupadd -g $GID $USERNAME && \
    useradd -u $UID -g $GID -m -s /bin/bash $USERNAME && \
    mkdir -p /home/$USERNAME/models && \
    chown -R $USERNAME:$USERNAME /home/$USERNAME 

# 3. Build block: Install build-deps, compile, install, and purge
RUN pacman -S --noconfirm base-devel boost git cmake ninja rust && \
    git clone --recursive https://github.com/FastFlowLM/FastFlowLM.git /tmp/FastFlowLM && \
    cd /tmp/FastFlowLM/src && \
    # We install directly to the user's home to keep it "mildly portable"
    cmake --preset linux-default -G Ninja \
        -DCMAKE_CXX_FLAGS="-march=native" \
        -DXRT_DIR="/usr" && \
    cmake --build build -j$(nproc) && \
    cmake --install build && \
    cd / && \
    rm -rf /tmp/FastFlowLM && \
    pacman -Rs --noconfirm base-devel git cmake ninja rust && \
    pacman -Scc --noconfirm

USER $USERNAME
WORKDIR /home/$USERNAME