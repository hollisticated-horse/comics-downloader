# TODO

## Completed
- [x] Add `readcomicsonline.ru` scraper (separate from `readcomiconline.li`)
- [x] Remove `readcomiconline.li` scraper: site uses server-side encrypted obfuscation, permanently broken (`pkg/sites/readcomicsonline.go`) with data-src primary path, JS `var pages` fallback, issue listing, `--all`, `--last` support, and full test coverage
- [x] Convert Makefile to Justfile
- [x] Add GUI unit tests (`cmd/gui/gui_test.go`) and `test-gui` Justfile recipe
