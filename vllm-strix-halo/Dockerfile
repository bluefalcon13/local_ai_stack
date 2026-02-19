ARG VERSION=7.12.0a20260218
ARG AMDGPU_FAMILY=gfx1151
ARG RELEASE_TYPE=nightlies

FROM ubuntu:24.04 AS get_ready_for_the_shit_show
ARG VERSION
ARG AMDGPU_FAMILY
ARG RELEASE_TYPE

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    g++ \
    gcc \
    git \
    libatomic1 \
    libgomp1 \
    python3 \
    python3-dev \
    python3-pip \
    python3-venv && \
    rm -fr /var/lib/apt/lists/*

ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH=/opt/venv/bin/:$PATH
RUN python -m pip install --upgrade pip wheel packaging "setuptools<80.0.0"

# INSTALL ROCM + COMAPANY
# RUN pip install --index-url https://repo.amd.com/rocm/whl/gfx1151/ \
#     torch torchvision torchaudio "rocm[libraries,devel]"
RUN pip install --no-cache-dir --pre \
    --index-url https://rocm.nightlies.amd.com/v2-staging/gfx1151/ \
    "torch==2.10.0+rocm${VERSION}" \
    "torchvision==0.25.0+rocm${VERSION}" \
    "torchaudio==2.10.0+rocm${VERSION}" \
    "rocm[devel,libraries]==${VERSION}"

RUN pip install /opt/venv/lib/python3.12/site-packages/_rocm_sdk_core/share/amd_smi

RUN python -m pip install --upgrade \
    "setuptools<80.0.0" \
    cmake \
    huggingface-hub[cli,hf_transfer] \
    ninja \
    numba \
    packaging \
    pip \
    scikit-build-core \
    scipy \
    setuptools-scm 

# clone all repos, in order to get their pip requriements
RUN git clone -b rocm_enabled https://github.com/ROCm/bitsandbytes.git && \
    git clone --recursive https://github.com/ROCm/flash-attention.git -b main_perf && \
    git clone --recursive https://github.com/ROCm/aiter.git && \
    git clone https://github.com/vllm-project/vllm.git /opt/vllm -b v0.16.0

# walk the repos, install their pip requirements, to ensure the pip venv
# structure is maintained, or we hope...
RUN cd ./aiter && pip install -r requirements-triton-comms.txt && \
    cd ../vllm && python ./use_existing_torch.py && \
    pip install -r ./requirements/rocm.txt

RUN rocm-sdk init

# --- THE STRIX HALO ARCHITECTURAL LOCK ---
# Hardcoding these because we know the truth now.
ENV ROCM_PATH=/opt/venv/lib/python3.12/site-packages/_rocm_sdk_devel
ENV CMAKE_PREFIX_PATH=$ROCM_PATH/lib/cmake \
    PATH=$ROCM_PATH/bin:$PATH

# --- COMPILER & ARCH DEFIANCE ---
ENV AMDGPU_TARGETS=gfx1151 \
    PYTORCH_ROCM_ARCH=gfx1151 \
    HIP_PLATFORM=amd \
    CXX=$ROCM_PATH/llvm/bin/clang++ \
    CC=$ROCM_PATH/llvm/bin/clang

# --- RAIN HELLFIRE UPON THEE, OFFLOAD-ARCH ---
# Because ROCm thinks building portable dockers is stupid,
# cause who would want to build portable, repeatable containers
# for their datacenter products? Surely the professionals have,
# this covered? What... some guy in his basement needs to fix this???
# I AM TIRED OF THESE LIES.
RUN find /opt/venv -name offload-arch -exec sh -c \
    'echo "Nuking $1..."; \
     mv "$1" "$1.orig"; \
     echo "#!/bin/sh\necho \"gfx1151\"" > "$1"; \
     chmod +x "$1"' _ {} \;

# Prep Goose's landing zone.
RUN mkdir -p /opt/wheels

# Build Aiter
FROM get_ready_for_the_shit_show AS aiter_shit_show

RUN cd /opt/aiter && FLASH_ATTENTION_TRITON_AMD_ENABLE="TRUE" \
    GPU_ARCHS="gfx1151" PYTORCH_ROCM_ARCH="gfx1151" \
    python3 setup.py bdist_wheel --dist-dir /opt/wheels

# Build flash_attn
FROM get_ready_for_the_shit_show AS flash_attn_shit_show

RUN cd flash-attention && \
    FLASH_ATTENTION_TRITON_AMD_ENABLE="TRUE" python3 setup.py bdist_wheel --dist-dir /opt/wheels

# Build bnb, cause why not...    
FROM get_ready_for_the_shit_show AS shit_show_as_a_bnb

# Explicitly use the Nightly Clang compiler
RUN cd ./bitsandbytes && \
    export BNB_ROCM_ARCH="gfx1151" && \
    export PYTORCH_ROCM_ARCH="gfx1151" && \
    cmake -S . -B build \
    -DCOMPUTE_BACKEND=hip \
    -DGPU_TARGETS="gfx1151" \
    -DCMAKE_HIP_COMPILER=$ROCM_PATH/llvm/bin/clang++ \
    -DCMAKE_CXX_COMPILER=$ROCM_PATH/llvm/bin/clang++ \
    -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build -j$(nproc)

RUN cd ./bitsandbytes && python3 setup.py bdist_wheel --dist-dir /opt/wheels

# Build vLLM wheel for final stage...
# Did I say build, I meant wrestle with
# Godzilla and his rediculously complex
# build detection system...
FROM get_ready_for_the_shit_show AS the_vllm_show

WORKDIR /opt/vllm

# 1. Bring in the trifecta of custom wheels
COPY --from=aiter_shit_show /opt/wheels /opt/wheels
COPY --from=flash_attn_shit_show /opt/wheels /opt/wheels
COPY --from=shit_show_as_a_bnb /opt/wheels /opt/wheels

RUN pip install --no-cache-dir /opt/wheels/*.whl

ENV VLLM_GPU_ARCH="gfx1151" \
    PYTORCH_ROCM_ARCH="gfx1151" \
    VLLM_SKIP_CUDART_CHECK=1
RUN python3 setup.py bdist_wheel --dist-dir /opt/wheels

FROM ubuntu:24.04 AS mordor

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    g++ \
    gcc \
    libatomic1 \
    libelf1 \
    libncurses6 \
    libnuma1 \
    python3-dev \
    python3-pip \
    python3 \
    python3-venv && \
    rm -rf /var/lib/apt/lists/*

# Setup the Nightly Repo (Using the 0217 path as the baseline)
# cause almost only counts for horseshoes, handgrenades, and
# rocm builds?
RUN echo "deb [arch=amd64 trusted=yes] https://rocm.nightlies.amd.com/deb/20260217-22086179463/ stable main" \
    > /etc/apt/sources.list.d/rocm-nightly.list \
    && apt-get update && apt-get install -y --no-install-recommends \
    # The architecture-locked meta-package for the Strix Halo
    amdrocm7.12-gfx1151 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=get_ready_for_the_shit_show /opt/venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH" \
    PYTHONUNBUFFERED=1

RUN echo "/opt/rocm/core-7.12/lib" > /etc/ld.so.conf.d/rocm.conf && \
    echo "/opt/rocm/core-7.12/lib64" >> /etc/ld.so.conf.d/rocm.conf && \
    echo "/opt/rocm/core-7.12/share/amd_smi/amdsmi" >> /etc/ld.so.conf.d/rocm.conf && \
    echo "/opt/rocm/core-7.12/llvm/lib" >> /etc/ld.so.conf.d/rocm.conf && \
    ldconfig


COPY --from=the_vllm_show /opt/wheels /opt/wheels
RUN pip install --no-cache-dir /opt/wheels/*.whl && \
    rm -fr /opt/wheels

# I dislike hardcoding into a runtime a bunch of args:
# So here are a few youd probably like to run...
# HSA_OVERRIDE_GFX_VERSION=11.5.1
# AITER_DISABLE_JIT=1
# VLLM_USE_ROCM_AITER=1
# VLLM_ROCM_USE_AITER_MLA=1

ENV VLLM_TARGET_DEVICE="rocm" \
    FLASH_ATTENTION_TRITON_AMD_ENABLE="TRUE"

# The "Flash-ROCm" Identity Hack
# Forces the ROCm flash_attn to be discoverable by vLLM's Qwen3 implementation
RUN ln -s /opt/venv/lib/python3.12/site-packages/flash_attn_2_cuda.cpython-312-x86_64-linux-gnu.so \
          /opt/venv/lib/python3.12/site-packages/flash_attn_2_rocm.so || true

WORKDIR /opt
EXPOSE 8000

ENV AITER_JIT_DIR=/tmp/aiter_jit_show \
    CC=amdclang \
    CXX=amdclang++ \
    VLLM_ROCM_USE_AITER_MHA=0
RUN mkdir -p $AITER_JIT_DIR/build && \
    apt update && apt install amdrocm-core-dev-gfx1151 -y && \
    rm -rf /var/lib/apt/lists/* && \
    pip install "mistral-common>=1.8.6" timm pillow

COPY --from=aiter_shit_show /opt/aiter /opt/aiter

ENTRYPOINT ["vllm", "serve"]