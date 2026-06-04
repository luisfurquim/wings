//go:build js && wasm

package wings

import (
	"reflect"
	"testing"
)

// attrMap builds the getAttr closure resolveTrigger expects from a plain map,
// returning "" for absent attributes (mirroring attrVal on a real element).
func attrMap(m map[string]string) func(string) string {
	return func(name string) string { return m[name] }
}

func TestResolveTrigger(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]string
		event string
		want  []triggerRoute
	}{
		{
			name:  "no channels: nothing routes",
			attrs: map[string]string{},
			event: "save",
			want:  []triggerRoute{},
		},
		{
			name:  "specific handler only",
			attrs: map[string]string{"@save": "onSave"},
			event: "save",
			want:  []triggerRoute{{handler: "onSave"}},
		},
		{
			name:  "else fires for an un-wired event",
			attrs: map[string]string{"@else": "onElse"},
			event: "save",
			want:  []triggerRoute{{handler: "onElse", prependEvent: true}},
		},
		{
			name:  "specific wins over else for its own event",
			attrs: map[string]string{"@save": "onSave", "@else": "onElse"},
			event: "save",
			want:  []triggerRoute{{handler: "onSave"}},
		},
		{
			name:  "else handles the other event when a specific exists elsewhere",
			attrs: map[string]string{"@save": "onSave", "@else": "onElse"},
			event: "cancel",
			want:  []triggerRoute{{handler: "onElse", prependEvent: true}},
		},
		{
			name:  "all spies on a wired event, after the specific handler",
			attrs: map[string]string{"@save": "onSave", "@all": "spy"},
			event: "save",
			want: []triggerRoute{
				{handler: "onSave"},
				{handler: "spy", prependEvent: true},
			},
		},
		{
			name:  "all alone fires for any event",
			attrs: map[string]string{"@all": "spy"},
			event: "whatever",
			want:  []triggerRoute{{handler: "spy", prependEvent: true}},
		},
		{
			name:  "un-wired event: else then all, in that order",
			attrs: map[string]string{"@else": "onElse", "@all": "spy"},
			event: "cancel",
			want: []triggerRoute{
				{handler: "onElse", prependEvent: true},
				{handler: "spy", prependEvent: true},
			},
		},
		{
			name:  "all three: specific primary, all last, else suppressed",
			attrs: map[string]string{"@save": "onSave", "@else": "onElse", "@all": "spy"},
			event: "save",
			want: []triggerRoute{
				{handler: "onSave"},
				{handler: "spy", prependEvent: true},
			},
		},
	}
	for _, c := range cases {
		got := resolveTrigger(attrMap(c.attrs), c.event)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: resolveTrigger = %+v, want %+v", c.name, got, c.want)
		}
	}
}
