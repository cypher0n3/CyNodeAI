# Development Documentation Directory

## Purpose

This directory serves as a **temporary working space** for documenting development activities, decisions, and analysis during active feature development.

**IMPORTANT:** All files in this directory (except this README) **MUST be cleaned up before merging to the main branch**.
This is not a permanent documentation location.

### Key Characteristics

- All files here (except this README) are **temporary** and must be cleaned up before merging
- This is **not** for permanent project documentation
- Files here document the "how and why" of development work in progress

## Pre-Merge Cleanup Requirement

Before merging any branch to main, complete these steps:

1. Review all files in `dev_docs/`
2. Decide the fate of each document:
    - Move valuable content to appropriate permanent location in `docs/` or otherwise
    - Delete temporary working documents
    - Extract and preserve important decisions or rationale
3. Ensure only `README.md` remains in this directory

Branches with unreviewed files in `dev_docs/` will be rejected during merge review.

## File Naming Convention

To maintain clarity and organization, use descriptive filenames with dates:

### Recommended Format

```text
YYYY-MM-DD_document_type_description.md
```

### File Name Components

- **Date:** ISO format (YYYY-MM-DD) for when the document was created
- **Document Type:** Clear type identifier (see examples below)
- **Description:** Specific, descriptive summary of content
- **Extension:** `.md` (Markdown format)

### Example Filenames

Good examples:

- `2024-01-15_analysis_mlkem_implementation_gaps.md`
- `2024-01-15_plan_path_normalization_refactor.md`
- `2024-01-15_notes_quantum_encryption_decisions.md`
- `2024-01-15_review_file_validation_changes.md`

Less helpful examples:

- `plan.md` - Missing date and context
- `notes.md` - Too generic, difficult to find later
- `temp.md` - Unclear purpose

### Common Document Types

- `analysis` - Code analysis, gap analysis, dependency mapping
- `plan` - Development plans, architecture decisions, design notes
- `notes` - Implementation notes, technical decisions, progress tracking
- `review` - Code reviews, check-in reports, validation results
- `findings` - Test results, coverage reports, BDD findings
- `issues` - Bug reports, troubleshooting notes, resolution tracking

### For Human Developers

This directory is available for your temporary development notes:

- Working documents during feature development
- Analysis notes and research
- Design explorations and prototypes
- Meeting notes related to development work

Remember: move valuable content to permanent locations before merging.

### For AI Assistants

If you're an AI coding assistant creating development documentation, follow the naming convention above to ensure your documentation is easily traceable and reviewable.
Human developers can review these documents to understand the reasoning behind changes.

When making commits, document your work in this directory:

- Create a dated document explaining your analysis and decisions
- Include what you changed and why
- Note any tradeoffs or alternatives considered
- Reference relevant specifications from `docs/tech_specs/`, requirements from `docs/requirements`, and/or feature files from `features`

See [`ai_files/ai_coding_instructions.md`](../ai_files/ai_coding_instructions.md) for detailed guidelines.
