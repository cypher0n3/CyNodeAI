# Feature Specifications

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How To Use These Features](#3-how-to-use-these-features)
- [4 Testing And Validation](#4-testing-and-validation)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains Gherkin `.feature` files that describe acceptance-level behavior for CyNodeAI.
These files are intended to be readable by humans and usable as executable specifications when a BDD runner is wired into the repo.

Treat feature files as a high-level contract for system behavior rather than as implementation notes.

## 2 What This Directory Contains

Feature files in this directory describe end-to-end flows across the orchestrator and worker node components.

## 3 How to Use These Features

Use these files as a reference when implementing endpoints, workflows, and integration behavior.
Keep scenarios aligned with the technical specs and with the actual implementation.

If a scenario becomes outdated, update the feature file and the corresponding tests together.

## 4 Testing and Validation

This repository currently validates behavior primarily through Go tests and end-to-end developer tooling.

- Run `just test-go` to run Go tests across all modules.
- Run `just e2e` to run the repository happy path that exercises orchestrator and worker node behavior.

Feature scenarios may also be reflected in orchestrator integration tests under [`orchestrator/internal/handlers/`](../orchestrator/internal/handlers/).

## 5 Cross-References

- Root project overview at [`README.md`](../README.md).
- Technical specifications index at [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).
- Orchestrator implementation at [`orchestrator/README.md`](../orchestrator/README.md).
- Worker node implementation at [`worker_node/README.md`](../worker_node/README.md).
