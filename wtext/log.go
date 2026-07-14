package wtext

import "github.com/luisfurquim/goose"

// G is the logger for this module. Errors are visible by default: the
// guard recovers report through Logf(1), and a swallowed panic with no
// trace is undebuggable.
//
// It lives in a file without a build tag because the portable half logs
// too: the style-library parser drops a hostile entry and says so, the
// way the document sheet's adoption does.
var G goose.Alert

func init() { G.Set(1) }
