Package htmltmpl is Go 1.20.4's html/template package, slightly modified.

Modifications:

* The `package` lines have changed to reflect the new name, as have some test strings.
* It imports `internal/text/template{,/parse}` instead of `text/template{,/parse}`.
* It defines some types to use the original html/template package, for user convenience.
* Some gofumpt reformatting might have occurred. :)
