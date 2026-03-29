# Historical Design Archive

This directory is the index for historical design material that is no longer the active implementation or planning direction for Grael.

At the moment, the main historical topic is the old idea that Grael would contain its own built-in memory layer.

That is no longer true.

The memory/knowledge product now lives separately as `OmnethDB`.

---

## What Is Historical

The following documents still contain historical memory-system material:

- [ARCHITECTURE.md](../ARCHITECTURE.md)
- [ARCHITECTURE_ADDENDUM.md](../ARCHITECTURE_ADDENDUM.md)
- [ARCHITECTURE_CORRECTIONS.md](../ARCHITECTURE_CORRECTIONS.md)
- [GRAEL_RUNTIME_SPEC.md](../GRAEL_RUNTIME_SPEC.md)

Those files remain in place because they still contain useful runtime history and non-memory design context.

Their memory sections should not be read as active Grael roadmap or active Grael implementation scope.

---

## Active Source Of Truth

For current Grael planning and implementation, prefer:

1. [V1_SCOPE.md](../V1_SCOPE.md)
2. [V1_CANONICAL_BASELINE.md](../V1_CANONICAL_BASELINE.md)
3. [adr/0015-memory-layer-belongs-to-omnethdb-not-grael.md](../adr/0015-memory-layer-belongs-to-omnethdb-not-grael.md)
4. [OMNETHDB_BOUNDARY.md](../OMNETHDB_BOUNDARY.md)

---

## Why Keep The Historical Material

We keep it because:

- it explains how the original thinking evolved
- it may still be useful when shaping OmnethDB
- it preserves architectural history instead of rewriting it away

But keeping it is not permission to treat it as current Grael scope.
