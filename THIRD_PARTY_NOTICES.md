# Third-Party Notices

This file records third-party software and static anatomy material redistributed
or consumed by BodySense. The software and anatomy asset license boundaries are
separate and must not be conflated.

## Vanatome software

Project: Vanatome
Source: https://github.com/vixotic/Vanatome
Initial BodySense package pins from ADR 0006: `@vixotic/vanatome-react` 0.1.6 and
`@vixotic/vanatome-atlas` 0.1.4. The Web dependency lockfile is authoritative
once the viewer lane installs these packages.
License: MIT

Vanatome's MIT license applies to its viewer/software. It does **not** relicense
the anatomy atlas assets described below.

MIT License

Copyright (c) 2026 Vanatome contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Vanatome Human Atlas 1.4.0 / Z-Anatomy-derived material

Provider: Vanatome Human Atlas
Atlas release: `1.4.0`
Atlas build ID: `994e6cc8ffbb212e`
Source catalog: https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json
Primary anatomy source: https://github.com/Z-Anatomy/Models
License: Creative Commons Attribution-ShareAlike 4.0 International
License: https://creativecommons.org/licenses/by-sa/4.0/

Upstream attribution credits Z-Anatomy contributors, including Gauthier Kervyn
and Marcin Zielinski, with additional contributors and upstream sources
recorded by Z-Anatomy. Z-Anatomy also documents material derived from
BodyParts3D; see the upstream Z-Anatomy provenance and
https://dbarchive.biosciencedbc.jp/en/bodyparts3d/download.html.

Vanatome describes its atlas adaptations as including web export, stable
structure/hierarchy identifiers, material adjustments, curve-to-mesh
conversion, geometry optimization, and system-bundle partitioning.

### BodySense redistribution statement

BodySense mirrors the 26 pinned upstream release files for atlas 1.4.0
byte-for-byte. BodySense does not modify the upstream catalog, metadata,
attribution, or GLB bytes. The mirrored upstream hierarchy is preserved inside
a BodySense-controlled version directory so the original relative catalog
references resolve unchanged.

BodySense adds only redistribution/provenance material and a copy of the CC BY-SA
4.0 legal code. These additions do not relicense the atlas.

Integrity and inventory are defined by:

`./scripts/anatomy/vanatome-1.4.0.manifest.json`

The Web image redistributes the release under:

`/static/anatomy/vanatome/1.4.0/`

The catalog consumed by the Viewer is:

`/static/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json`

The packaged release also contains the upstream `ATTRIBUTION.txt`,
`BODYSENSE_PROVENANCE.txt`, and `LICENSES/CC-BY-SA-4.0.txt`.
