package live

import (
	"encoding/json"
	"strings"
	"time"
)

// DefaultTransitionDuration_ms is the default duration, in milliseconds, of live.JS transitions.
const DefaultTransitionDuration = 200 * time.Millisecond

type cmd struct {
	kind string
	args any
}

// JS provides a way to precompose simple client-side DOM changes that don't require a round-trip to the server.
// Create a JS struct, call its methods to build up a command, and then render it as the value of a phx-* attribute.
type JS struct {
	cmds []*cmd
}

// Hide hides elements.
// When the action is triggered on the client, phx:hide-start is dispatched to the hidden elements. After the time specified by :time, phx:hide-end is dispatched.
func (js *JS) Hide(opts *HideOpts) *JS {
	js.add("hide", opts)
	return js
}

// Show shows elements.
// When the action is triggered on the client, phx:show-start is dispatched to the shown elements. After the time specified by :time, phx:show-end is dispatched.
func (js *JS) Show(opts *ShowOpts) *JS {
	js.add("show", opts)
	return js
}

// Toggle toggles element visibility.
// When the toggle is complete on the client, a phx:show-start or phx:hide-start, and phx:show-end or phx:hide-end event will be dispatched to the toggled elements.
func (js *JS) Toggle(opts *ToggleOpts) *JS {
	js.add("toggle", opts)
	return js
}

// Push pushes an event to the server.
func (js *JS) Push(event string, opts *PushOpts) *JS {
	type pushOpts struct {
		Event string `json:"event"`
		PushOpts
	}
	pos := &pushOpts{
		Event:    event,
		PushOpts: *opts,
	}
	js.add("push", pos)
	return js
}

func (js *JS) add(kind string, options any) {
	js.cmds = append(js.cmds, &cmd{
		kind: kind,
		args: options,
	})
}

func (js *JS) MarshalJSON() ([]byte, error) {
	var o [][]any
	for _, op := range js.cmds {
		o = append(o, []any{
			op.kind,
			op.args,
		})
	}
	return json.Marshal(o)
}

func (js *JS) String() string {
	s, err := js.MarshalJSON()
	if err != nil {
		panic(err)
	}
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

func (t Transition) MarshalJSON() ([]byte, error) {
	tc, sc, ec := []byte(`[]`), []byte(`[]`), []byte(`[]`)
	var err error
	if t.TransitionClass != "" {
		tc, err = json.Marshal(strings.Split(t.TransitionClass, " "))
		if err != nil {
			return nil, err
		}
	}
	if t.StartClass != "" {
		sc, err = json.Marshal(strings.Split(t.StartClass, " "))
		if err != nil {
			return nil, err
		}
	}
	if t.EndClass != "" {
		ec, err = json.Marshal(strings.Split(t.EndClass, " "))
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal([3]json.RawMessage{tc, sc, ec})
}

type HideOpts struct {
	// To is the DOM selector of the element to hide, or empty to target the interacted element.
	To string
	// Transition to apply, if any.
	Transition *Transition
	// Time is the duration of the transition, if present; defaults to DefaultTransitionDuration if 0.
	Time time.Duration
}

func (o *HideOpts) MarshalJSON() ([]byte, error) {
	type hideOpts struct {
		To         *string    `json:"to"`
		Transition Transition `json:"transition"`
		Time       int64      `json:"time"`
	}
	ho := &hideOpts{
		Time: DefaultTransitionDuration.Milliseconds(),
	}
	if o.To != "" {
		ho.To = &o.To
	}
	if o.Transition != nil {
		ho.Transition = *o.Transition
	}
	if o.Time != 0 {
		ho.Time = o.Time.Milliseconds()
	}
	return json.Marshal(ho)
}

type ShowOpts struct {
	// To is the DOM selector of the element to show, or empty to target the interacted element.
	To string
	// Transition to apply, if any.
	Transition *Transition
	// Time is the duration of the transition, if present; defaults to DefaultTransitionDuration if 0.
	Time time.Duration
	// Display is the CSS display value to set when showing; defaults to "block".
	Display string
}

func (o *ShowOpts) MarshalJSON() ([]byte, error) {
	type showOpts struct {
		To         *string    `json:"to"`
		Transition Transition `json:"transition"`
		Time       int64      `json:"time"`
		Display    string     `json:"display"`
	}
	so := &showOpts{
		Display: "block",
		Time:    DefaultTransitionDuration.Milliseconds(),
	}
	if o.To != "" {
		so.To = &o.To
	}
	if o.Transition != nil {
		so.Transition = *o.Transition
	}
	if o.Time != 0 {
		so.Time = o.Time.Milliseconds()
	}
	if o.Display != "" {
		so.Display = o.Display
	}
	return json.Marshal(so)
}

type ToggleOpts struct {
	// To is the DOM selector of the element to toggle visibility of, or empty to target the interacted element.
	To string
	// In is the transition to apply when showing the element.
	In *Transition
	// Out is the transition to apply when hiding the element.
	Out *Transition
	// Time is the duration of the transition, if present; defaults to DefaultTransitionDuration if 0.
	Time time.Duration
	// Display is the CSS display value to set when showing; defaults to "block".
	Display string
}

func (o *ToggleOpts) MarshalJSON() ([]byte, error) {
	type toggleOpts struct {
		To      *string    `json:"to"`
		In      Transition `json:"ins"`
		Out     Transition `json:"outs"`
		Time    int64      `json:"time"`
		Display string     `json:"display"`
	}
	to := &toggleOpts{
		Display: "block",
		Time:    DefaultTransitionDuration.Milliseconds(),
	}
	if o.To != "" {
		to.To = &o.To
	}
	if o.In != nil {
		to.In = *o.In
	}
	if o.Out != nil {
		to.Out = *o.Out
	}
	if o.Time != 0 {
		to.Time = o.Time.Milliseconds()
	}
	if o.Display != "" {
		to.Display = o.Display
	}
	return json.Marshal(to)
}

type PushOpts struct {
	// Target is the selector or component ID to push to
	Target string `json:"target,omitempty"`
	// Loading is the selector to apply the phx loading classes to
	Loading string `json:"loading,omitempty"`
	// PageLoading is a boolean indicating whether to trigger the "phx:page-loading-start"
	// and "phx:page-loading-stop" events. Defaults to `false`
	PageLoading bool `json:"page_loading,omitempty"`
	// Value is optional data to include in the event's `value` property
	Value any `json:"value,omitempty"`
}
