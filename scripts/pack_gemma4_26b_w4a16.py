#!/usr/bin/env python3
"""Pack Google's cached QAT Q4_0 unquantized Gemma 4 26B-A4B checkpoint into W4A16 compressed-tensors."""

import json
import os
import shutil
import struct
import sys
from pathlib import Path

import torch
from huggingface_hub import snapshot_download
from llmcompressor import oneshot
from llmcompressor.modifiers.quantization import QuantizationModifier
from transformers import AutoProcessor, AutoTokenizer


def resolve_model_cls():
    import transformers

    for attr in (
        "AutoModelForImageTextToText",
        "AutoModelForMultimodalLM",
        "Gemma4ForConditionalGeneration",
    ):
        cls = getattr(transformers, attr, None)
        if cls is not None:
            return cls
    raise ImportError("Could not find a Gemma 4 compatible AutoModel class in transformers")


def resolve_snapshot_dir(model_id: str) -> str:
    try:
        return snapshot_download(model_id, local_files_only=True)
    except Exception:
        cache_dir = Path.home() / ".cache" / "huggingface" / "hub"
        repo_dir = cache_dir / ("models--" + model_id.replace("/", "--"))
        snapshots = sorted((repo_dir / "snapshots").glob("*")) if (repo_dir / "snapshots").exists() else []
        if snapshots:
            return str(snapshots[-1])
        raise


def verify_packed_output(output_dir: Path) -> None:
    safetensor_files = sorted(output_dir.glob("*.safetensors"))
    if not safetensor_files:
        print(f"ERROR: No .safetensors files written to {output_dir}", file=sys.stderr)
        sys.exit(1)

    total_bytes = sum(p.stat().st_size for p in safetensor_files)
    total_gib = total_bytes / (1024 ** 3)

    packed_count = 0
    expert_packed_count = 0
    for st_path in safetensor_files:
        with open(st_path, "rb") as f:
            hdr_len = struct.unpack("<Q", f.read(8))[0]
            hdr = json.loads(f.read(hdr_len))
        for key in hdr:
            if key == "__metadata__":
                continue
            if key.endswith(".weight_packed"):
                packed_count += 1
                if "experts" in key:
                    expert_packed_count += 1

    cfg_path = output_dir / "config.json"
    qcfg = {}
    if cfg_path.exists():
        qcfg = json.loads(cfg_path.read_text(encoding="utf-8")).get("quantization_config", {})

    print("\n=== Verification Summary ===")
    print(f"Output directory       : {output_dir}")
    print(f"Total .safetensors size: {total_gib:.2f} GiB ({len(safetensor_files)} shard(s))")
    print(f"Total weight_packed    : {packed_count}")
    print(f"Expert weight_packed   : {expert_packed_count}")
    print(f"quantization_config    : {bool(qcfg)} (method={qcfg.get('quant_method')}, format={qcfg.get('format')})")

    if expert_packed_count == 0:
        print(
            "\nWARNING: 0 expert tensors were packed! MoE experts may have been skipped by llmcompressor.",
            file=sys.stderr,
        )
    if total_gib > 25.0:
        print(
            f"\nWARNING: Output size ({total_gib:.2f} GiB) exceeds 25 GiB — MoE experts were likely left in BF16!",
            file=sys.stderr,
        )


def main() -> None:
    model_id = os.environ.get("MODEL_ID", "google/gemma-4-26B-A4B-it-qat-q4_0-unquantized")
    output_dir = Path(os.environ.get("OUTPUT_DIR", "/out"))
    group_size = int(os.environ.get("GROUP_SIZE", "32"))

    output_dir.mkdir(parents=True, exist_ok=True)

    model_cls = resolve_model_cls()
    print(f"Loading model '{model_id}' via {model_cls.__name__} from local cache on CPU (BF16)...")
    try:
        model = model_cls.from_pretrained(
            model_id,
            dtype=torch.bfloat16,
            device_map="cpu",
            local_files_only=True,
        )
    except TypeError:
        model = model_cls.from_pretrained(
            model_id,
            torch_dtype=torch.bfloat16,
            device_map="cpu",
            local_files_only=True,
        )

    try:
        processor = AutoProcessor.from_pretrained(model_id, local_files_only=True)
    except Exception:
        processor = AutoTokenizer.from_pretrained(model_id, local_files_only=True)

    recipe = QuantizationModifier(
        config_groups={
            "group_0": {
                "targets": ["Linear"],
                "weights": {
                    "num_bits": 4,
                    "type": "int",
                    "symmetric": True,
                    "strategy": "group",
                    "group_size": group_size,
                    "dynamic": False,
                    "observer": "memoryless_minmax",
                },
                "input_activations": None,
                "output_activations": None,
            }
        },
        ignore=[
            "lm_head",
            "re:.*embed.*",
            "re:.*router.*",
            "re:.*vision_tower.*",
            "re:.*audio_tower.*",
        ],
    )

    print(f"Packing weights to W4A16 compressed-tensors (group_size={group_size}, data-free QAT grid)...")
    result = oneshot(model=model, recipe=recipe)
    if result is not None:
        model = result

    print(f"Saving compressed checkpoint to {output_dir}...")
    model.save_pretrained(str(output_dir), save_compressed=True)
    processor.save_pretrained(str(output_dir))

    # Ensure chat_template.jinja is preserved for vLLM gemma4 tool/reasoning parsers.
    target_template = output_dir / "chat_template.jinja"
    if not target_template.exists():
        try:
            snap_dir = Path(resolve_snapshot_dir(model_id))
            src_template = snap_dir / "chat_template.jinja"
            if src_template.exists():
                shutil.copy2(src_template, target_template)
                print(f"Copied chat_template.jinja from {src_template}")
        except Exception as exc:
            print(f"Note: could not copy chat_template.jinja automatically: {exc}", file=sys.stderr)

    verify_packed_output(output_dir)


if __name__ == "__main__":
    main()
