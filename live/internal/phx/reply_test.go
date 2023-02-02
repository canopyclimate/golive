package phx

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/canopyclimate/golive/internal/tmpl"
	"github.com/dsnet/try"
)

func TestHeartbeat(t *testing.T) {

	msgRef := "1"
	hb := fmt.Sprintf(`[null,%q,"phoenix","phx_reply",{"response":{},"status":"ok"}]`, msgRef)

	hbr := NewHeartbeat(msgRef)

	b, err := hbr.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if hb != string(b) {
		t.Fatalf("got \n%q want \n%q", string(b), hb)
	}

}

func TestRendered(t *testing.T) {

	msg := Msg{
		JoinRef: "4",
		MsgRef:  "4",
		Topic:   "phoenix",
	}

	rendered := []byte(`{"0":"phx-879983f9-81be-4b7a-89b8-e59d7d76bbc9"}`)
	res := fmt.Sprintf(`[%q,%q,%q,"phx_reply",{"response":{"rendered":%s},"status":"ok"}]`, msg.JoinRef, msg.MsgRef, msg.Topic, rendered)

	r := NewRendered(msg, rendered)

	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if res != string(b) {
		t.Fatalf("got \n\n%q want \n\n%q", string(b), res)
	}

}

func TestReplyDiff(t *testing.T) {

	msg := Msg{
		JoinRef: "4",
		MsgRef:  "4",
		Topic:   "phoenix",
	}

	diff := []byte(`{"0":"phx-879983f9-81be-4b7a-89b8-e59d7d76bbc9"}`)
	res := fmt.Sprintf(`[%q,%q,%q,"phx_reply",{"response":{"diff":%s},"status":"ok"}]`, msg.JoinRef, msg.MsgRef, msg.Topic, diff)

	r := NewReplyDiff(msg, diff)

	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if res != string(b) {
		t.Fatalf("got \n\n%q want \n\n%q", string(b), res)
	}

}

func TestUploadReplyDiff(t *testing.T) {

	msg := Msg{
		JoinRef: "4",
		MsgRef:  "4",
		Topic:   "phoenix",
	}

	tr := tmpl.Tree{
		Dynamics: []any{
			"dyn",
		},
		Statics: []string{
			"static",
			"",
		},
	}

	type constraints struct {
		Accept      []string
		MaxEntries  int   `json:"max_entries"`
		MaxFileSize int64 `json:"max_file_size"`
		ChunkSize   int64 `json:"chunk_size"`
	}
	c := constraints{
		MaxEntries:  1,
		MaxFileSize: 100,
	}

	type entry struct {
		Size int64
		Name string
	}

	entries := []entry{
		{
			Name: "foo",
			Size: 10,
		},
	}

	diffJson := try.E1(tr.JSON())

	configJson := try.E1(json.Marshal(c))
	entriesJson := try.E1(json.Marshal(entries))

	res := fmt.Sprintf(`[%q,%q,%q,"phx_reply",{"response":{"diff":%s,"config":%s,"entries":%s},"status":"ok"}]`, msg.JoinRef, msg.MsgRef, msg.Topic, diffJson, string(configJson), string(entriesJson))

	r := NewUploadReplyDiff(msg, diffJson, configJson, entriesJson)

	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if res != string(b) {
		t.Fatalf("got \n\n%q want \n\n%q", string(b), res)
	}

}

func TestDiff(t *testing.T) {

	topic := "lv:phx-asfdasdfa"

	diff := []byte(`{"0":"phx-879983f9-81be-4b7a-89b8-e59d7d76bbc9"}`)
	res := fmt.Sprintf(`[null,null,%q,"diff",%s]`, topic, diff)

	r := NewDiff(nil, topic, diff)

	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if res != string(b) {
		t.Fatalf("got \n\n%q want \n\n%q", string(b), res)
	}

}

func TestEmptyReply(t *testing.T) {

	msg := Msg{
		JoinRef: "4",
		MsgRef:  "4",
		Topic:   "phoenix",
	}

	res := fmt.Sprintf(`[%q,%q,%q,"phx_reply",{"response":{},"status":"ok"}]`, msg.JoinRef, msg.MsgRef, msg.Topic)

	r := NewEmptyReply(msg)

	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if res != string(b) {
		t.Fatalf("got \n\n%q want \n\n%q", string(b), res)
	}

}
