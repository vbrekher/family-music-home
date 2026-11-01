# Local deepmerge security patch

Based on the readable CommonJS distribution of deepmerge v4.3.1 (MIT), https://github.com/TehShrike/deepmerge/tree/v4.3.1. Original copyright and license are retained. This is a local Family Music Home patch, not an official upstream version.

CVE-2026-93753 / GHSA-gcqx-w48m-w2h4 affects keys copied from either input, including a target with its own `__proto__` property or a source merged into a null-prototype target. `getKeys` now excludes `__proto__`, `constructor` and `prototype` for both input-copy paths. Ordinary keys, symbols, arrays, custom merge and clone options retain the upstream implementation. Configurations intentionally using those three property names must rename them.

Regression tests exercise hostile JSON and null-prototype objects at the root, nested objects and array elements. This package version identifies the downstream patch; it must not be described as a published upstream fix.
