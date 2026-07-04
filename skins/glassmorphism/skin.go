//go:build js && wasm

// Package glassmorphism provides the "glassmorphism" wings skin bundle.
//
// It combines two orthogonal component skins into a single activation:
//   - [glass] (CategoryAtmosphere) — backdrop-filter blur + translucent surface alpha
//   - [glasslighting] (CategoryLighting) — diagonal shimmer gradient + diffused halo shadow
//
// Usage:
//
//	_ = wings.ApplySkin("glassmorphism")
//
// Power-user alternative — apply components individually to mix with other skins:
//
//	_ = wings.ApplySkin("glass")         // blur only
//	_ = wings.ApplySkin("glasslighting") // gradient/shadow only
//
// The bundle declares the union of both components' categories, so it
// conflicts with any active Atmosphere or Lighting skin.
package glassmorphism

import (
	_ "embed"

	"github.com/luisfurquim/wings"
	"github.com/luisfurquim/wings/skins/glass"
	"github.com/luisfurquim/wings/skins/glasslighting"
)

func init() {
	wings.RegisterSkin(
		"glassmorphism",
		glass.Categories|glasslighting.Categories,
		glass.CSS+glasslighting.CSS,
	)
}
