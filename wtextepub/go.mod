module github.com/luisfurquim/wings/wtextepub

go 1.25.0

// The wings require lags on purpose: this module needs
// EditorCore.Content()/MenuPlugin, which ship in the release AFTER v0.19.0
// — until then it builds inside the wings workspace only. Bump wings here
// when tagging wtextepub (prefixed tag: wtextepub/vX.Y.Z).
require (
	github.com/luisfurquim/ugarit v0.0.1
	github.com/luisfurquim/wings v0.19.0
	golang.org/x/net v0.55.0
)

require (
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/StefanSchroeder/Golang-Roman v1.0.0 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/luisfurquim/goose v0.1.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)
