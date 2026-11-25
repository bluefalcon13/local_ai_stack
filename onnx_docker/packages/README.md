# AMD Ryzen AI & XRT Packages

This directory houses the proprietary Debian packages required to build the `onnx-worker` container for the AMD Ryzen AI XDNA 2 NPU (e.g. Strix Point / Strix Halo).

These packages are proprietary and require an active AMD Developer portal login to download. They are explicitly excluded from Git (`.gitignore`).

---

### Required Packages:

1. **AMD XRT (Xilinx Runtime)**
   - `xrt_*_24.04-amd64-base.deb`
   - `xrt_*_24.04-amd64-base-dev.deb`
   - `xrt_*_24.04-amd64-npu.deb`
   - `xrt_plugin.*.release_24.04-amd64-amdxdna.deb`

2. **Ryzen AI Runtime**
   - `ryzen-ai-runtime_1.9.0-1_amd64.deb` (or current 1.9.x release)

---

### Download Instructions:

1. Sign in to the [AMD Developer Portal](https://www.amd.com/en/developer/resources/ryzen-ai-software.html).
2. Download the Ubuntu 24.04 `.deb` packages for **XRT** and **Ryzen AI Runtime**.
3. Place all `.deb` files directly into this directory (`onnx_docker/packages/`).
4. Build the worker container:
   ```bash
   docker compose build onnx-worker
   ```
