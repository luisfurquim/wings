module github.com/luisfurquim/wings

go 1.25.0

toolchain go1.26.4

require github.com/luisfurquim/goose v0.1.0

require (
	github.com/fsnotify/fsnotify v1.10.1
	golang.org/x/crypto v0.51.0
	golang.org/x/mod v0.35.0
	golang.org/x/net v0.55.0
	golang.org/x/text v0.37.0
)

require golang.org/x/sys v0.45.0 // indirect

// These releases wrongly shipped the local-dev go.work in the module zip; its
// sub-module `use` entries are missing from the zip, so building gen_i18n from
// the cached wings tree fails. Fixed in v0.16.5-alpha (go.work untracked).
retract (
	v0.16.4-alpha
	v0.16.3-alpha
)
