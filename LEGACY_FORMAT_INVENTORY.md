# Legacy format inventory

The original ZawodyWeb packages use XML manifests named `contest.xml`,
`serie.xml`, `problem.xml`, and `tests.xml`, with referenced input/output and
statement files. Required inventory fields include contest and series ordering,
problem code/title, statement/PDF references, time/memory limits, test order,
points, language identifiers, and checker identifiers.

Cotests currently accepts only bounded flat `.in`/`.out` ZIP pairs for private
draft tests. It rejects traversal, symlinks, unmatched pairs, unsupported files
and excessive compressed or extracted content. It does not parse legacy XML,
execute checker code, or claim checker/language compatibility.

Unknown language and checker identifiers are retained in revision metadata for
review. Representative production legacy exports are still required before an
XML mapping can be implemented and verified.
