# Documentation

- [Entry Points](#entry-points)
- [Other Directories](#other-directories)
- [Relation to Repo](#relation-to-repo)
- [See Also](#see-also)

## Entry Points

This directory holds the main project documentation for CyNodeAI.

- **Requirements ("what")**: [requirements/](requirements/README.md) - canonical normative requirements; one file per domain.
- **Technical specifications ("how")**: [tech_specs/_main.md](tech_specs/_main.md) - design and implementation guidance; traces back to requirements.
- **MVP and phases**: [mvp.md](mvp.md) - minimum viable product scope and phased plan.
- **Development setup**: [development_setup.md](development_setup.md) - how to run the stack locally and run tests.
- **Open WebUI integration**: [openwebui_cynodeai_integration.md](openwebui_cynodeai_integration.md) - connect Open WebUI to the User API Gateway (OpenAI-compatible).

## Other Directories

- [docs_standards/](docs_standards/README.md) - Markdown and spec authoring conventions.
- [examples/](examples/README.md) - example configuration files (e.g. bootstrap YAML).

## Relation to Repo

See the root [meta.md](../meta.md) for project summary, repository layout, and contribution expectations.
Requirements take precedence over tech specs and code; tech specs take precedence over code.

## See Also

- Root [README.md](../README.md) - project overview, development setup, architecture.
- [dev_docs/](../dev_docs/README.md) - temporary development notes (clean up before merge).
- [features/](../features/README.md) - Gherkin feature files and BDD specs.
- [scripts/](../scripts/README.md) - helper scripts for setup and systemd unit generation.
- [.ci_scripts/](../.ci_scripts/README.md) - CI and doc-validation scripts.
