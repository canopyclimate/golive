Package htmltmpl is Go 1.20.4's html/template package, slightly modified.

Modifications:

* The `package` lines have changed to reflect the new name, as have some test strings.
* It imports `internal/text/template` instead of `text/template`.
* It defines some types to use the original html/template package, for user convenience.
