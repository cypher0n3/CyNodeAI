# Markdown Conventions

## Overview

This document defines Markdown authoring conventions for the CyNodeAI repository.

It applies to all Markdown files unless a document explicitly states an exception.

## Formatting Rules

- Use proper Markdown headings.
  Do not use pseudo-headings (for example `**Label:**`).
- Use "=>" instead of arrows.
- Avoid non-ASCII characters except:
  - work-tracking docs under [`dev_docs/`](../../dev_docs/) where ✅, ❌, 📊, ⚠️ are allowed.
  - `README.md` files, which may use emoji, badges, and other visual elements.
- Include a blank line after every heading.
- Include a blank line before and after every list.
- Avoid using Markdown tables; prefer lists or separate sections instead.
- Put one sentence per line.
  Tables may contain multiple sentences in a single cell.
- Indent list-item code blocks by four spaces.
- When referencing a file or path within this repository, it must be a Markdown link.
- Avoid content duplication.
  Prefer links to the single source of truth.

### Inline Code (Backticks)

Use inline code spans to mention code or literals in prose.

If the inline code itself must contain backticks, use a longer backtick delimiter.

This is wrong:

```markdown
`Something \`backticked\` more stuff`
```

This is right:

```markdown
``Something `backticked` more stuff``
```

To mention code fence markers (for example, ```go) in normal prose, wrap them in inline code so they are not parsed as a code block:

````markdown
` ```go ` code blocks
````

### Escaping Parentheses

Parentheses typically do not need escaping in normal Markdown prose.
Avoid unnecessary escapes unless required by a link destination or a validator.

## Headings

This section expands the heading rules enforced by the documentation validation pipeline.

### Heading Depth

- Avoid heading depth beyond H5 (`#####`).
- H6 (`######`) and deeper headings are flagged by validation.

### Heading Uniqueness

- Headings must be unique within a document.
- Validators treat headings as duplicates if their text matches after stripping any leading numbering prefix.
  For example, `## 1 Overview` and `## 2 Overview` are duplicates.

### Heading Numbering

When a heading uses a numbering prefix, the numbering must be consistent with the heading level.

- H2 headings use one numeric segment: `## 1 Title`
- H3 headings use two numeric segments: `### 1.1 Title`
- H4 headings use three numeric segments: `#### 1.1.1 Title`
- H5 headings use four numeric segments: `##### 1.1.1.1 Title`

Additional rules:

- Numbers must be sequential within each parent section.
- Child heading numbers must match their parent section number.
- H2 numbering punctuation must be consistent within the file.
  Do not mix `## 1 Title` and `## 1. Title`.
- Do not use parentheses for heading numbering.
  This is wrong: `## 3) Heading Text`

### Table of Contents

When a document includes a Table of Contents, place it under the H1 and before the first H2.

- There must be no content below the H1 except the TOC.
  Do not add intro text or any other content between the H1 and the first H2; only the TOC link list belongs there.
- Do not use an H2 heading for the TOC.
  The TOC is the link list that follows the H1; the first `##` in the document is the first content section.
- All document content must appear under an H2 or deeper heading.
  No prose or other content may sit at the document root (between the TOC and the next section); everything belongs under a section heading.

Validation enforces this via the `no-h1-content` markdownlint rule.

### Heading Text

Validation recommends Title Case for headings and can report suggestions.

Rules:

- Prefer Title Case for heading text.
- Preserve the exact text inside backticks.
- Avoid single-word headings unless they are genuinely meaningful.
- Avoid organizational headings that have no content.
  The `no-empty-heading` markdownlint rule enforces that H2+ have at least one line of content directly under them (before any subheading); content under subheadings does not count.
  Blank and HTML-comment-only lines do not count.
  Suppress per section with `<!-- no-empty-heading allow -->` on its own line.

## Inline HTML

Inline HTML is generally discouraged.

If a document requires inline HTML for narrowly-scoped anchors, the document must specify the exact allowed forms and validation must enforce them.

For spec anchor exceptions, see:

- [`spec_authoring_writing_and_validation.md`](./spec_authoring_writing_and_validation.md)

## Validation Workflow

Use the project [`justfile`](../../justfile) to validate docs.

- Run `just lint-md` (or `just lint-md 'path/to/file.md'`) after creating or changing Markdown.
- Fix any reported issues and re-run until the check passes.

For the full validation pipeline and the check ordering rationale, see:

- [`spec_authoring_writing_and_validation.md`](./spec_authoring_writing_and_validation.md)
