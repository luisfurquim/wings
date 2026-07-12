module github.com/luisfurquim/wings/wtextepub

go 1.25.0

// Nested module: releases use the directory-prefixed tag (wtextepub/vX.Y.Z),
// and the wings require must first ship the contract this module builds on
// (EditorCore.Content, MenuPlugin) — bump it before tagging.
require (
	github.com/luisfurquim/ugarit v0.0.2
	github.com/luisfurquim/wings v0.22.0
	golang.org/x/net v0.55.0
)

require (
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/StefanSchroeder/Golang-Roman v1.0.0 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/luisfurquim/goose v0.1.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)
