# Durable record storage

Projects, issues and comments store metadata as readable block YAML at the start
of `content.md`, between lines containing `---`. The Markdown body follows the
closing delimiter's newline. For example:

```markdown
---
created_at: "2026-09-30T00:00:00.123456Z"
id: "0347a340-1eed-42e4-9392-d3b13b18a921"
repository_refs: []
revision: 1
schema_version: 1
title: "test/poc-3"
updated_at: "2026-09-30T00:00:00.123456Z"
---
trying the lit workflows
```

The service writes UTC timestamps with six fractional digits (microseconds).
The reader accepts supported RFC3339 timestamp precision without rewriting it.
The writer uses deterministic key ordering and two-space indentation. String
values are double quoted, including timestamps and strings such as `true`,
`null` or `123`, so their types and text remain exact. Integer revisions never
pass through floating-point conversion. Embedded newlines in metadata strings
are escaped; they cannot become front-matter delimiters. Body bytes, including
Unicode, CRLF, blank lines and the absence of a final newline, are preserved.

Readers require block YAML metadata. Earlier compact JSON and root flow mappings
are rejected as unsupported, including when preceded by YAML comments. There is
no migration or automatic deletion of old test data. Startup and reads do not
rewrite records; no-op mutations and unrelated records retain their existing
bytes. Schema version remains 1.
Relationship files, immutable history entries, journals and canonical request
hash inputs remain JSON. Accepted history is not rewritten.

YAML supports comments, block mappings and sequences within a deliberately narrow
metadata data model. Duplicate or unknown fields, missing required fields,
incorrect scalar types, invalid UTF-8, unsupported schema versions, non-string
mapping keys, anchors, aliases, merge keys, explicit tags, multiple YAML documents
and implicit timestamp/float values are rejected. Timestamps must be quoted
strings. Nullable assignees and null collections retain their existing
meaning; scalar fields such as IDs and timestamps cannot be null. Integer values
must use decimal notation. The body is never parsed as YAML.

The codec uses the YAML organization's maintained stable
[`go.yaml.in/yaml/v3`](https://pkg.go.dev/go.yaml.in/yaml/v3) node API, validates
nodes before conversion, and retains the protocol's strict JSON validation at
the typed boundary. This avoids YAML's automatic coercion into Go string fields.

Use the service API or `lit` to mutate accepted records. Editing files behind
a running service bypasses its concurrency and history guarantees; startup
validation rejects records that disagree with accepted history.
