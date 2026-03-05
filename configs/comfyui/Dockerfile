FROM docker.io/kyuz0/amd-strix-halo-comfyui:latest AS myplusone
ARG UID=1000
ARG GID=1000
ARG USERNAME=ai-svc

RUN groupadd -g ${GID} ${USERNAME} || true && \
    useradd -u ${UID} -g ${GID} -m -s /bin/bash ${USERNAME} && \
    mkdir -p /home/${USERNAME}/comfy-models/ && \
    chown -R ${USERNAME}:${USERNAME} /home/${USERNAME} && \
    chmod -R 700 /home/${USERNAME}

ENV VIRTUAL_ENV=/opt/venv
ENV PATH=/opt/venv/bin:$PATH
ENV PIP_NO_CACHE_DIR=1

ENV ROCM_VERSION=7.12.0a20260211

RUN pip install --index-url https://rocm.nightlies.amd.com/v2/gfx1151/ --pre \
  "rocm-sdk-devel==$ROCM_VERSION" && rocm-sdk init

ENV ROCM_PATH=/opt/venv/lib64/python3.13/site-packages/_rocm_sdk_devel
ENV HIP_PATH=$ROCM_PATH \
    PATH=$ROCM_PATH/bin:$PATH \
    PYTORCH_ROCM_ARCH=gfx1151 \
    FLASH_ATTENTION_TRITON_AMD_ENABLE=TRUE

RUN echo "$ROCM_PATH/lib" > /etc/ld.so.conf.d/rocm.conf && \
    ldconfig

RUN pip install sageattention --no-build-isolation

USER ${USERNAME}

ENTRYPOINT ["/bin/bash", "-c", "/opt/set_extra_paths.sh && exec \"$@\"", "--"]