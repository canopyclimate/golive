package live

import (
	"testing"
	"time"
)

type jsTestCase struct {
	expected string
	js       *JS
}

func runCases(t *testing.T, cases []jsTestCase) {
	for _, c := range cases {
		str := c.js.String()
		if c.expected != str {
			t.Fatalf("got \n%q want \n%q", str, c.expected)
		}
	}
}

func TestShowCmd(t *testing.T) {
	cases := []jsTestCase{
		{
			js:       (&JS{}).Show(&ShowOpts{To: "#selector"}),
			expected: `[["show",{"to":"#selector","transition":[[],[],[]],"time":200,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Show(&ShowOpts{To: "#selector", Transition: &Transition{TransitionClass: "class1 class2"}}),
			expected: `[["show",{"to":"#selector","transition":[["class1","class2"],[],[]],"time":200,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Show(&ShowOpts{To: "#selector", Time: 1000 * time.Millisecond}),
			expected: `[["show",{"to":"#selector","transition":[[],[],[]],"time":1000,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Show(&ShowOpts{Display: "inline"}),
			expected: `[["show",{"to":null,"transition":[[],[],[]],"time":200,"display":"inline"}]]`,
		},
	}

	runCases(t, cases)
}

func TestHideCmd(t *testing.T) {
	cases := []jsTestCase{
		{
			js:       (&JS{}).Hide(&HideOpts{To: "#selector"}),
			expected: `[["hide",{"to":"#selector","transition":[[],[],[]],"time":200}]]`,
		},
		{
			js:       (&JS{}).Hide(&HideOpts{To: "#selector", Transition: &Transition{TransitionClass: "class1 class2"}}),
			expected: `[["hide",{"to":"#selector","transition":[["class1","class2"],[],[]],"time":200}]]`,
		},
		{
			js:       (&JS{}).Hide(&HideOpts{To: "#selector", Time: 1000 * time.Millisecond}),
			expected: `[["hide",{"to":"#selector","transition":[[],[],[]],"time":1000}]]`,
		},
	}

	runCases(t, cases)
}

func TestToggleCmd(t *testing.T) {
	cases := []jsTestCase{
		{
			js:       (&JS{}).Toggle(&ToggleOpts{To: "#selector"}),
			expected: `[["toggle",{"to":"#selector","ins":[[],[],[]],"outs":[[],[],[]],"time":200,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Toggle(&ToggleOpts{To: "#selector", In: &Transition{TransitionClass: "class1 class2"}}),
			expected: `[["toggle",{"to":"#selector","ins":[["class1","class2"],[],[]],"outs":[[],[],[]],"time":200,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Toggle(&ToggleOpts{To: "#selector", Out: &Transition{StartClass: "class1 class2"}}),
			expected: `[["toggle",{"to":"#selector","ins":[[],[],[]],"outs":[[],["class1","class2"],[]],"time":200,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Toggle(&ToggleOpts{To: "#selector", Time: 1000 * time.Millisecond}),
			expected: `[["toggle",{"to":"#selector","ins":[[],[],[]],"outs":[[],[],[]],"time":1000,"display":"block"}]]`,
		},
		{
			js:       (&JS{}).Toggle(&ToggleOpts{To: "#selector", Display: "inline"}),
			expected: `[["toggle",{"to":"#selector","ins":[[],[],[]],"outs":[[],[],[]],"time":200,"display":"inline"}]]`,
		},
	}

	runCases(t, cases)
}

func TestPushCmd(t *testing.T) {
	type testValue struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	cases := []jsTestCase{
		{
			js:       (&JS{}).Push("event", &PushOpts{}),
			expected: `[["push",{"event":"event"}]]`,
		},
		{
			js:       (&JS{}).Push("event", &PushOpts{Target: "#selector"}),
			expected: `[["push",{"event":"event","target":"#selector"}]]`,
		},
		{
			js:       (&JS{}).Push("event", &PushOpts{Loading: "#loading_selector"}),
			expected: `[["push",{"event":"event","loading":"#loading_selector"}]]`,
		},
		{
			js:       (&JS{}).Push("event", &PushOpts{PageLoading: true}),
			expected: `[["push",{"event":"event","page_loading":true}]]`,
		},
		{
			js:       (&JS{}).Push("event", &PushOpts{Value: "value"}),
			expected: `[["push",{"event":"event","value":"value"}]]`,
		},
		{
			js:       (&JS{}).Push("event", &PushOpts{Value: testValue{A: "a", B: 1}}),
			expected: `[["push",{"event":"event","value":{"a":"a","b":1}}]]`,
		},
	}

	runCases(t, cases)
}
