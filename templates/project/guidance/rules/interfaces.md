# Interfaces and migrations

When changing a public API, command, schema, configuration format or persisted
record, inspect its callers and compatibility expectations. Keep machine-readable
formats versioned where consumers need to distinguish contracts. Separate public
behaviour from implementation details.

Handle invalid input with an actionable failure, preserve user data and make
partial failure observable. Treat migrations, deprecations and destructive cleanup
as explicit scoped work. Verify old/new behaviour where compatibility is promised;
state intentional incompatibilities rather than claiming automatic portability.

Do not embed a particular framework, agent or OS in a general interface unless
it is the purpose of an explicitly optional adapter. Document actual runtime
dependencies separately from the languages and tools used by consumer projects.
