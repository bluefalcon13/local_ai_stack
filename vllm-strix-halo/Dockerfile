ARG VERSION=7.12.0a20260218
ARG AMDGPU_FAMILY=gfx1151
ARG RELEASE_TYPE=nightlies

FROM ubuntu:24.04 AS builder
ARG VERSION
ARG AMDGPU_FAMILY
ARG RELEASE_TYEP

WORKDIR /opt
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

ENV VIRTUAL_ENV=/opt/venv
RUN uv venv --python 3.13 --seed $VIRTUAL_ENV
ENV PATH=/opt/venv/bin/:$PATH
RUN python -m pip install --upgrade pip wheel packaging "setuptools<80.0.0"

# INSTALL ROCM + COMAPANY
RUN pip install --index-url https://repo.amd.com/rocm/whl/gfx1151/ \
    torch torchvision torchaudio "rocm[libraries,devel]"
# RUN pip install --no-cache-dir --pre \
#     --index-url https://rocm.nightlies.amd.com/v2-staging/gfx1151/ \
#     "torch==2.10.0+rocm${VERSION}" \
#     "torchvision==0.25.0+rocm${VERSION}" \
#     "torchaudio==2.10.0+rocm${VERSION}" \
#     "rocm[devel,libraries]==${VERSION}"

ENV TORCH_HIP_ARCH_LIST="gfx1151"
ENV PYTORCH_ROCM_ARCH="gfx1151"

RUN rocm-sdk init
# path extracted via docker cli using `rocm-sdk path --root`
RUN ln -s /opt/venv/lib/python3.13/site-packages/_rocm_sdk_devel /opt/rocm
ENV ROCM_PATH=/opt/rocm
ENV PATH=$ROCM_PATH/bin:$PATH

ENV HIP_PLATFORM=amd
ENV HIP_PATH=$ROCM_PATH
ENV HIP_CLANG_PATH=$ROCM_PATH/llvm/bin
ENV HIP_INCLUDE_PATH=$ROCM_PATH/include
ENV HIP_LIB_PATH=$ROCM_PATH/lib
ENV HIP_DEVICE_LIB_PATH=$ROCM_PATH/lib/llvm/amdgcn/bitcode
ENV PATH="$ROCM_PATH/bin:$HIP_CLANG_PATH:$PATH"
ENV HSA_NO_SCRATCH_RECLAIM=1

RUN echo "${ROCM_PATH}/lib" > /etc/ld.so.conf.d/rocm.conf && \
    ldconfig

RUN pip install ${ROCM_PATH}/share/amd_smi
ENV HIP_VISIBLE_DEVICES=-1
ENV ROCR_VISIBLE_DEVICES=-1
ENV ROCM_VISIBLE_DEVICES=-1

# ROYALLY, AND OFFICIALLY, FUCK YOU VLLM BUILD SYSTEM. IS IT REALLY THAT
# DIFFICULT TO BE HAPPY THAT SOMEONE TOLD YOU TO BUILD FOR A SINGLE DEVICE???
RUN mv /opt/venv/bin/offload-arch /opt/venv/bin/offload-arch.orig && \
    echo '#!/bin/sh' > /opt/venv/bin/offload-arch && \
    echo 'echo "gfx1151"' >> /opt/venv/bin/offload-arch && \
    chmod 777 /opt/venv/bin/offload-arch && \
    mv /opt/venv/lib/python3.13/site-packages/_rocm_sdk_core/lib/llvm/bin/offload-arch \
    /opt/venv/lib/python3.13/site-packages/_rocm_sdk_core/lib/llvm/bin/offload-arch.orig && \
    cp /opt/venv/bin/offload-arch /opt/venv/lib/python3.13/site-packages/_rocm_sdk_core/lib/llvm/bin/

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    g++ \
    gcc \
    git \
    libatomic1 \
    libgomp1 && \
    rm -fr /var/lib/apt/lists/*

ENV DEVICE_LIB_PATH=/opt/rocm/amdgpu/bitcode

RUN python -m pip install --upgrade \
    cmake \
    ninja \
    packaging \
    pip \
    scikit-build-core \
    setuptools-scm \
    "setuptools<80.0.0"



# --- BITSANDBYTES (Strix Halo Optimized) ---
WORKDIR /opt
RUN git clone -b rocm_enabled https://github.com/ROCm/bitsandbytes.git
WORKDIR /opt/bitsandbytes
ENV PATH="/opt/rocm/llvm/bin:$PATH"

# Identity Fix for ROCm Nightlies
ENV HIP_PLATFORM="amd"
ENV CMAKE_PREFIX_PATH="/opt/rocm"

# Explicitly use the Nightly Clang compiler
RUN cmake -S . \
    -DGPU_TARGETS="gfx1151" \
    -DBNB_ROCM_ARCH="gfx1151" \
    -DCOMPUTE_BACKEND=hip \
    -DCMAKE_HIP_COMPILER=/opt/rocm/llvm/bin/clang++ \
    -DCMAKE_CXX_COMPILER=/opt/rocm/llvm/bin/clang++ && \
    make -j$(nproc) && \
    pip install . --no-build-isolation 

WORKDIR /opt

RUN git clone --recursive https://github.com/ROCm/flash-attention.git -b main_perf && \
    cd flash-attention && \
    FLASH_ATTENTION_TRITON_AMD_ENABLE="TRUE" python setup.py install

RUN git clone --recursive https://github.com/ROCm/aiter.git && \
    cd /opt/aiter && \
    export GPU_ARCHS="gfx1151" && \
    python3 setup.py develop && \
    pip install -r requirements-triton-comms.txt

RUN git clone https://github.com/vllm-project/vllm.git /opt/vllm
WORKDIR /opt/vllm

# ENV VLLM_TARGET_DEVICE="rocm"
# ENV PYTORCH_ROCM_ARCH="gfx1151"
# ENV AMDGPU_TARGETS="gfx1151"
# ENV HIP_ARCHITECTURES="gfx1151"
# THIS IS THE BUILD LINE THAT NEVER ENDS!
# YES, IT GOES ON AND ON, MY FRIEND!
# SOME PEOPLE STARTED BUILDING IT, NOT KNOWING WHAT IT WAS!
# AND THEY WILL CONTINUE BUILDING IT FOREVER CAUSE THERE IS
# TOO DAMN MANY BUILD ARGS TO BUILD VLLM FOR ROCM!
# 7. Build vLLM (Wheel Method) with CLANG Host Compiler
RUN python -m pip install --upgrade numba \
    setuptools_scm scipy huggingface-hub[cli,hf_transfer] && \
    python ./use_existing_torch.py && \
    pip install -r ./requirements/rocm.txt

RUN export CMAKE_ARGS="-DCMAKE_PREFIX_PATH=$ROCK_PATH/lib/cmake \
        -DAMDGPU_TARGETS=gfx1151 \
        -DHIP_ARCHITECTURES=gfx1151 \
        -DTorch_DIR=/opt/venv/lib/python3.13/site-packages/torch/share/cmake/Torch \
        -DCMAKE_HIP_COMPILER=/opt/rocm/llvm/bin/clang++ \
        -DCMAKE_CXX_COMPILER=/opt/rocm/llvm/bin/clang++" && \
    python setup.py develop

RUN mv /opt/venv/bin/offload-arch.orig /opt/venv/bin/offload-arch && \
    mv /opt/venv/lib/python3.13/site-packages/_rocm_sdk_core/lib/llvm/bin/offload-arch.orig \
    /opt/venv/lib/python3.13/site-packages/_rocm_sdk_core/lib/llvm/bin/offload-arch

WORKDIR /opt
ENV HIP_VISIBLE_DEVICES=
ENV ROCR_VISIBLE_DEVICES=
ENV ROCM_VISIBLE_DEVICES=

ENTRYPOINT ["vllm", "serve"]
CMD ["/bin/bash"]

# 10. Force Upgrade Transformers (User Override)
# Required for GLM Flash. vLLM reports incompatibility with transformers >= 5, 
# but this version (5.0.0) has been tested and confirmed working.
# RUN python -m pip install transformers==5.0.0



# # Final build, minus all the cruft
# FROM ubuntu:24.04 AS final
# COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# ENV VIRTUAL_ENV=/opt/venv
# RUN uv venv --python 3.13 --seed $VIRTUAL_ENV
# ENV PATH=/opt/venv/bin/:$PATH

# WORKDIR /opt

# COPY --from=builder /tmp/dist/*.whl /tmp/dist/

# RUN apt-get update && apt-get install -y --no-install-recommends \
#     build-essential \
#     ca-certificates \
#     g++ \
#     gcc \
#     git \
#     libatomic1 \
#     libgomp1 && \
#     rm -fr /var/lib/apt/lists/*

# RUN pip install --no-cache-dir --pre \
#     --index-url https://rocm.nightlies.amd.com/v2-staging/gfx1151/ \
#     "torch==2.10.0+rocm${VERSION}" \
#     "torchvision==0.25.0+rocm${VERSION}" \
#     "torchaudio==2.10.0+rocm${VERSION}" \
#     "rocm[devel,libraries]==${VERSION}"

# RUN rocm-sdk init && \
#     ln -s /opt/venv/lib/python3.13/site-packages/_rocm_sdk_devel /opt/rocm
# ENV ROCM_PATH=/opt/rocm
# ENV ROCM_HOME=/opt/rocm
# ENV PYTORCH_ROCM_ARCH="gfx1151"
# # ENV HIP_PATH=/opt/rocm
# ENV PATH=$ROCM_PATH/bin:$PATH

# RUN echo "${ROCM_PATH}/lib" > /etc/ld.so.conf.d/rocm.conf && \
#     ldconfig

# RUN python -m pip install ray
# RUN python -m pip install $ROCM_PATH/share/amd_smi

# RUN pip install /tmp/dist/bitsandbytes*.whl

# # manually reinstall aiter cause wheels just dont cut it on this
# # boat!
# RUN git clone --recursive https://github.com/ROCm/aiter.git && \
#     cd /opt/aiter && \
#     export GPU_ARCHS="gfx1151" && \
#     pip install -e . && \
#     pip install -r requirements-triton-comms.txt

# RUN export VLLM_TARGET_DEVICE=rocm && \
#     pip install /tmp/dist/vllm*.whl

# # 10. Force Upgrade Transformers (User Override)
# # Required for GLM Flash. vLLM reports incompatibility with transformers >= 5, 
# # but this version (5.0.0) has been tested and confirmed working.
# RUN python -m pip install transformers==5.0.0

# RUN chmod -R a+rwX /opt

