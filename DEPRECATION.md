# Crosswind deprecation

Active evaluation development and the shared scenario authoring contract are owned
by CompFly Platform. Make new scenario contract changes in Platform's
`svc/agent-eval/services/api/pkg/scenario`; use its schema/validation endpoints or
`cmd/scenario-contract` CLI. The schema URI remains `compfly://scenarios/v1/schema`.

The contract was extracted from `e020d387ccac54f3e9006c0584a92f2879485b63`
without changing schema bytes, fixture semantics, or revision digests. Crosswind's
`api/pkg/scenario` and CLI remain frozen compatibility snapshots for previously
pinned consumers. Do not dual-write new features to both repositories.

Platform still depends on the older Go `api/pkg/api` facade and Python Crosswind
runtime. Keep these available while their consumers are migrated. This deprecation
notice does not archive the repository, delete packages, or remove deployments.

Remaining migration work: move required services/models and Python runtime code
into Platform-owned packages, update consumers and build pipelines, validate
behavior, then remove the remaining dependency pins. Only retire old runtime
infrastructure after its users have migrated.
