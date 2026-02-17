# Go Shared Libraries

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How To Use In Go Modules](#3-how-to-use-in-go-modules)
- [4 Testing And Linting](#4-testing-and-linting)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains Go packages that are shared between the orchestrator and worker node modules.
It is intended to hold stable contracts and small cross-cutting utilities that must compile in multiple services.

If a shared type is part of a network boundary, treat it as an API contract and evolve it carefully to avoid breaking dependent services.

## 2 What This Directory Contains

This directory is a standalone Go module defined by [`go.mod`](go.mod).

- [`contracts/`](contracts/): Shared contract packages.
- [`contracts/workerapi/`](contracts/workerapi/): Types used to communicate with the worker API.
- [`contracts/nodepayloads/`](contracts/nodepayloads/): Types used for node registration and capability payloads.
- [`contracts/problem/`](contracts/problem/): Shared error and problem response types.

The orchestrator and worker node modules depend on this module via local replaces in their respective `go.mod` files.

## 3 How to Use in Go Modules

In this repository, other Go modules import this module using the canonical module path and a local replace.
See the replace directives in [`orchestrator/go.mod`](../orchestrator/go.mod) and [`worker_node/go.mod`](../worker_node/go.mod).

When adding or changing contracts, prefer additive changes and maintain backward compatibility where possible.
When a breaking change is required, update all dependent modules in the same change series.

## 4 Testing and Linting

All Go modules in this repository are checked by repo-level `just` targets.

- Run `just test-go` to run Go tests across all modules.
- Run `just lint-go` or `just lint-go-ci` to run Go lint checks across all modules.
- Run `just ci` to run the local CI suite (lint, tests with coverage, and vulnerability checks).

## 5 Cross-References

- Root project overview at [`README.md`](../README.md).
- Orchestrator implementation at [`orchestrator/README.md`](../orchestrator/README.md).
- Worker node implementation at [`worker_node/README.md`](../worker_node/README.md).
