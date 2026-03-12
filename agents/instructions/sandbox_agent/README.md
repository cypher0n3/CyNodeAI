# Sandbox Agent Instruction Bundle

## Contents

This directory contains the canonical baseline instruction bundle for the Sandbox Agent (SBA).
It can be supplied in the job payload (`context.baseline_context`) or baked into the SBA image.

Files are loaded in this order:

- [`01_baseline.md`](01_baseline.md)
- [`02_tools.md`](02_tools.md)

This bundle uses the same general format as the PMA instruction bundles.
See [docs/tech_specs/cynode_sba.md](../../../docs/tech_specs/cynode_sba.md).
