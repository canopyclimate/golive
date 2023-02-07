package live

import (
	"encoding/json"
	"strings"
)

// DefaultTransitionDuration_ms is the default duration, in milliseconds, of live.JS transitions.
const DefaultTransitionDuration_ms = int32(200)

type op struct {
	kind string
	args map[string]any
}

// JS provides a way to precompose simple client-side DOM changes that don't require a round-trip to the server.
// Create a JS struct, call its methods to build up a command, and then render it as the value of a phx-* attribute.
type JS struct {
	ops []*op
}

// Hide hides elements.
// When the action is triggered on the client, phx:hide-start is dispatched to the hidden elements. After the time specified by :time, phx:hide-end is dispatched.
func (js *JS) Hide(opts *HideOpts) {
	js.add("hide", opts)
}

// Show shows elements.
// When the action is triggered on the client, phx:show-start is dispatched to the shown elements. After the time specified by :time, phx:show-end is dispatched.
func (js *JS) Show(opts *ShowOpts) {
	js.add("show", opts)
}

// Toggle toggles element visibility.
// When the toggle is complete on the client, a phx:show-start or phx:hide-start, and phx:show-end or phx:hide-end event will be dispatched to the toggled elements.
func (js *JS) Toggle(opts *ToggleOpts) {
	js.add("toggle", opts)
}

type hasArgs interface {
	args() map[string]any
}

func (js *JS) add(kind string, a hasArgs) {
	js.ops = append(js.ops, &op{
		kind: kind,
		args: a.args(),
	})
}

func (js *JS) MarshalJSON() ([]byte, error) {
	var o [][]any
	for _, op := range js.ops {
		o = append(o, []any{
			op.kind,
			op.args,
		})
	}
	return json.Marshal(o)
}

func (js *JS) String() string {
	s, _ := js.MarshalJSON()
	return string(s)
}

// Transition describes a set of CSS class changes over time.
type Transition struct {
	// TransitionClass is the CSS transition class(es) to apply for the duration of the transition.
	TransitionClass string
	// StartClass is the CSS class(es) that apply at the start of a transition.
	StartClass string
	// EndClass is the CSS class(es) that apply at the end of a transition.
	EndClass string
}

type HideOpts struct {
	// To is the DOM selector of the element to hide, or empty to target the interacted element.
	To string
	// Transition to apply, if any.
	Transition *Transition
	// Time is the duration in milliseconds of the transition, if present; defaults to DefaultTransitionDuration_ms if 0.
	Time int32
}

func (o *HideOpts) args() map[string]any {
	var to *string
	time := DefaultTransitionDuration_ms
	transition := &Transition{}
	if o != nil {
		to = &o.To
		if o.Time != 0 {
			time = o.Time
		}
		if o.Transition != nil {
			transition = o.Transition
		}
	}
	return map[string]any{
		"time":       time,
		"to":         to,
		"transition": transition.phx(),
	}
}

type ShowOpts struct {
	// To is the DOM selector of the element to show, or empty to target the interacted element.
	To string
	// Transition to apply, if any.
	Transition *Transition
	// Time is the duration in milliseconds of the transition, if present; defaults to DefaultTransitionDuration_ms if 0.
	Time int32
	// Display is the CSS display value to set when showing; defaults to "block".
	Display string
}

func (o *ShowOpts) args() map[string]any {
	var to *string
	time := DefaultTransitionDuration_ms
	transition := &Transition{}
	display := "block"
	if o != nil {
		to = &o.To
		if o.Time != 0 {
			time = o.Time
		}
		if o.Transition != nil {
			transition = o.Transition
		}
		if o.Display != "" {
			display = o.Display
		}
	}
	return map[string]any{
		"time":       time,
		"to":         to,
		"transition": transition.phx(),
		"display":    display,
	}
}

type ToggleOpts struct {
	// To is the DOM selector of the element to toggle visibility of, or empty to target the interacted element.
	To string
	// In is the transition to apply when showing the element.
	In *Transition
	// Out is the transition to apply when hiding the element.
	Out *Transition
	// Time is the duration in milliseconds of the relevant transition, if present; defaults to DefaultTransitionDuration_ms if 0.
	Time int32
	// Display is the CSS display value to set when showing; defaults to "block".
	Display string
}

func (o *ToggleOpts) args() map[string]any {
	var to *string
	time := DefaultTransitionDuration_ms
	in := &Transition{}
	out := &Transition{}
	display := "block"
	if o != nil {
		to = &o.To
		if o.Time != 0 {
			time = o.Time
		}
		if o.In != nil {
			in = o.In
		}
		if o.Out != nil {
			out = o.Out
		}
		if o.Display != "" {
			display = o.Display
		}
	}
	return map[string]any{
		"time":    time,
		"to":      to,
		"ins":     in.phx(),
		"outs":    out.phx(),
		"display": display,
	}
}

func phxClasses(class string) []string {
	if class == "" {
		return []string{}
	}
	return strings.Split(class, " ")
}

func (t *Transition) phx() [3][]string {
	return [3][]string{
		phxClasses(t.TransitionClass),
		phxClasses(t.StartClass),
		phxClasses(t.EndClass),
	}
}
