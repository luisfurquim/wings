package wtextepub

import "github.com/luisfurquim/goose"

// G is this module's logger, at the same default level as wtext's: what
// an import silently could not do (a font the stores do not have, a
// stylesheet that did not fit) is exactly what someone staring at a book
// that came in wrong needs to read.
var G goose.Alert

func init() { G.Set(1) }
