#!/usr/bin/env python3
"""Generate the normalized BodySense inventory for the pinned Vanatome atlas.

The default sources are immutable files from the official Vanatome repository at
one exact commit. No npm/Vanatome runtime dependency is required.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any
from urllib.parse import urlparse
from urllib.request import Request, urlopen

ATLAS_RELEASE = "1.4.0"
UPSTREAM_COMMIT = "8185b3fa46a1dbefdd907390918c57954fd9b816"
OFFICIAL_CATALOG_URL = (
    "https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json"
)
RAW_BASE = (
    "https://raw.githubusercontent.com/vixotic/Vanatome/"
    f"{UPSTREAM_COMMIT}/public/atlas/demo-1.4.0"
)
DEFAULT_CATALOG_SOURCE = f"{RAW_BASE}/catalog.json"
DEFAULT_METADATA_SOURCE = f"{RAW_BASE}/full-body.metadata.json"
DEFAULT_OUTPUT = Path(__file__).resolve().parents[1] / "data" / (
    "vanatome-1.4.0-registry.generated.json"
)


def _read_bytes(source: str) -> bytes:
    parsed = urlparse(source)
    if parsed.scheme in {"http", "https"}:
        request = Request(
            source,
            headers={"User-Agent": "BodySense-Atlas-Inventory/1.0"},
        )
        with urlopen(request, timeout=30) as response:  # noqa: S310 - pinned HTTPS source
            return response.read()
    return Path(source).read_bytes()


def _load_json(source: str) -> tuple[dict[str, Any], str]:
    raw = _read_bytes(source)
    parsed = json.loads(raw)
    if not isinstance(parsed, dict):
        raise ValueError(f"Expected a JSON object from {source}")
    return parsed, hashlib.sha256(raw).hexdigest()


def _infer_laterality(anatomy_id: str, name: str) -> tuple[str | None, str | None]:
    evidence: list[tuple[str, str]] = []
    lower_id = anatomy_id.lower()
    lower_name = name.lower().strip()

    if lower_id.endswith("-left") or "-left-" in lower_id:
        evidence.append(("left", "anatomy_id"))
    if lower_id.endswith("-right") or "-right-" in lower_id:
        evidence.append(("right", "anatomy_id"))

    if lower_name.endswith(".l") or lower_name.startswith("left ") or " of left " in lower_name:
        evidence.append(("left", "name"))
    if lower_name.endswith(".r") or lower_name.startswith("right ") or " of right " in lower_name:
        evidence.append(("right", "name"))

    sides = {side for side, _ in evidence}
    if len(sides) != 1:
        return None, "conflicting_tokens" if len(sides) > 1 else None

    side = next(iter(sides))
    sources = "+".join(sorted({source for _, source in evidence}))
    return side, sources


def _catalog_full_body_bundle(catalog: dict[str, Any]) -> dict[str, Any]:
    profiles = catalog.get("profiles")
    bundles = catalog.get("bundles")
    if not isinstance(profiles, list) or not isinstance(bundles, list):
        raise ValueError("Catalog is missing profiles/bundles")

    profile = next(
        (item for item in profiles if item.get("id") == "full-body"),
        None,
    )
    if not profile:
        raise ValueError("Catalog is missing the full-body profile")

    bundle_id = profile.get("bundleId")
    bundle = next((item for item in bundles if item.get("id") == bundle_id), None)
    if not bundle:
        raise ValueError(f"Catalog is missing bundle {bundle_id!r}")
    return bundle


def generate_inventory(
    catalog: dict[str, Any],
    metadata: dict[str, Any],
    catalog_sha256: str,
    metadata_sha256: str,
) -> dict[str, Any]:
    atlas = catalog.get("atlas")
    if not isinstance(atlas, dict) or atlas.get("version") != ATLAS_RELEASE:
        raise ValueError(f"Catalog atlas version must be {ATLAS_RELEASE}")
    if metadata.get("atlasVersion") != ATLAS_RELEASE:
        raise ValueError(f"Metadata atlas version must be {ATLAS_RELEASE}")

    full_body = _catalog_full_body_bundle(catalog)
    if full_body.get("metadataUrl") != "./full-body.metadata.json":
        raise ValueError("full-body profile no longer points to full-body.metadata.json")
    if metadata.get("bundleId") != full_body.get("id"):
        raise ValueError("Catalog/metadata full-body bundle mismatch")
    if metadata.get("buildId") != atlas.get("buildId"):
        raise ValueError("Catalog/metadata build ID mismatch")

    structures = metadata.get("structures")
    if not isinstance(structures, list):
        raise ValueError("Metadata structures must be a list")
    if len(structures) != full_body.get("structureCount"):
        raise ValueError("Catalog structureCount does not match metadata")
    if metadata.get("nodeCount") != full_body.get("nodeCount"):
        raise ValueError("Catalog nodeCount does not match metadata")

    normalized: list[dict[str, Any]] = []
    seen_ids: set[str] = set()
    for structure in structures:
        anatomy_id = structure.get("id")
        name = structure.get("name")
        if not isinstance(anatomy_id, str) or not anatomy_id:
            raise ValueError("Every atlas structure needs a non-empty id")
        if anatomy_id in seen_ids:
            raise ValueError(f"Duplicate atlas structure id: {anatomy_id}")
        seen_ids.add(anatomy_id)
        if not isinstance(name, str) or not name:
            raise ValueError(f"Atlas structure {anatomy_id} is missing a name")

        laterality, laterality_evidence = _infer_laterality(anatomy_id, name)
        object_count = structure.get("objectCount", 0)
        if not isinstance(object_count, int) or object_count < 0:
            raise ValueError(f"Invalid objectCount for {anatomy_id}")

        position = structure.get("position")
        if position is not None:
            if (
                not isinstance(position, list)
                or len(position) != 3
                or any(not isinstance(value, (int, float)) for value in position)
            ):
                raise ValueError(f"Invalid focus position for {anatomy_id}")

        normalized.append(
            {
                "anatomyId": anatomy_id,
                "name": name,
                "kind": structure.get("kind"),
                "system": structure.get("system"),
                "layer": structure.get("layer"),
                "parent": structure.get("parentId"),
                "laterality": laterality,
                "lateralityEvidence": laterality_evidence,
                "focus": position,
                "selectable": bool(structure.get("selectable", False)),
                "geometry": {
                    "directMappedNodeCount": object_count,
                    "hasDirectGeometry": object_count > 0,
                },
            }
        )

    normalized.sort(key=lambda item: item["anatomyId"])
    mapped_structure_count = sum(
        1 for item in normalized if item["geometry"]["hasDirectGeometry"]
    )
    mapped_node_count = sum(
        item["geometry"]["directMappedNodeCount"] for item in normalized
    )

    return {
        "schemaVersion": 1,
        "atlasProvider": "vanatome",
        "atlasRelease": ATLAS_RELEASE,
        "atlasId": metadata.get("atlasId"),
        "catalogBuildId": atlas.get("buildId"),
        "bundleId": metadata.get("bundleId"),
        "source": {
            "officialCatalogUrl": OFFICIAL_CATALOG_URL,
            "upstreamRepository": "https://github.com/vixotic/Vanatome",
            "upstreamCommit": UPSTREAM_COMMIT,
            "catalogFixture": DEFAULT_CATALOG_SOURCE,
            "metadataFixture": DEFAULT_METADATA_SOURCE,
            "catalogSha256": catalog_sha256,
            "metadataSha256": metadata_sha256,
        },
        "summary": {
            "structureCount": len(normalized),
            "directGeometryStructureCount": mapped_structure_count,
            "mappedNodeCount": mapped_node_count,
            "fullBodyNodeCount": metadata.get("nodeCount"),
        },
        "structures": normalized,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--catalog", default=DEFAULT_CATALOG_SOURCE)
    parser.add_argument("--metadata", default=DEFAULT_METADATA_SOURCE)
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT))
    args = parser.parse_args()

    catalog, catalog_sha256 = _load_json(args.catalog)
    metadata, metadata_sha256 = _load_json(args.metadata)
    inventory = generate_inventory(
        catalog,
        metadata,
        catalog_sha256=catalog_sha256,
        metadata_sha256=metadata_sha256,
    )

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(inventory, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(
        f"wrote {output} with {inventory['summary']['structureCount']} structures, "
        f"{inventory['summary']['directGeometryStructureCount']} direct geometry IDs, "
        f"{inventory['summary']['mappedNodeCount']} mapped nodes"
    )


if __name__ == "__main__":
    main()
