package phx

import "encoding/json"

type Response struct {
	Rendered json.RawMessage `json:"rendered,omitempty"`
	Diff     json.RawMessage `json:"diff,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
	Entries  json.RawMessage `json:"entries,omitempty"`
}

type Payload struct {
	Response Response `json:"response"`
	Status   string   `json:"status"`
}

type Reply struct {
	JoinRef *string // nullable
	MsgRef  *string // nullable
	Topic   string
	Event   string
	Payload Payload
}

type Diff struct {
	JoinRef *string // nullable
	MsgRef  *string // nullable
	Topic   string
	Event   string
	Payload json.RawMessage
}

type Heartbeat struct {
	Reply
	Payload
}

func NewDiff(joinRef *string, topic string, diff []byte) *Diff {
	return &Diff{
		JoinRef: joinRef,
		Topic:   topic,
		Event:   "diff",
		Payload: diff,
	}
}

func NewEmptyReply(msg Msg) *Reply {
	return &Reply{
		JoinRef: &msg.JoinRef,
		MsgRef:  &msg.MsgRef,
		Topic:   msg.Topic,
		Event:   "phx_reply",
		Payload: Payload{
			Status: "ok",
		},
	}
}

func NewEmptyUploadReply(up UploadMsg) *Reply {
	return &Reply{
		JoinRef: &up.JoinRef,
		MsgRef:  &up.MsgRef,
		Topic:   up.Topic,
		Event:   "phx_reply",
		Payload: Payload{
			Status: "ok",
		},
	}
}

func NewUploadReplyDiff(msg Msg, diff []byte, config []byte, entries []byte) *Reply {
	return &Reply{
		JoinRef: &msg.JoinRef,
		MsgRef:  &msg.MsgRef,
		Topic:   msg.Topic,
		Event:   "phx_reply",
		Payload: Payload{
			Status: "ok",
			Response: Response{
				Diff:    diff,
				Config:  config,
				Entries: entries,
			},
		},
	}
}

func NewReplyDiff(msg Msg, diff []byte) *Reply {
	return &Reply{
		JoinRef: &msg.JoinRef,
		MsgRef:  &msg.MsgRef,
		Topic:   msg.Topic,
		Event:   "phx_reply",
		Payload: Payload{
			Status: "ok",
			Response: Response{
				Diff: diff,
			},
		},
	}
}

func NewRendered(msg Msg, rendered []byte) *Reply {
	return &Reply{
		JoinRef: &msg.JoinRef,
		MsgRef:  &msg.MsgRef,
		Topic:   msg.Topic,
		Event:   "phx_reply",
		Payload: Payload{
			Status: "ok",
			Response: Response{
				Rendered: rendered,
			},
		},
	}
}

func NewHeartbeat(msgRef string) *Reply {
	return &Reply{
		MsgRef: &msgRef,
		Topic:  "phoenix",
		Event:  "phx_reply",
		Payload: Payload{
			Status: "ok",
		},
	}
}

func (m *Reply) JSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Reply) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{m.JoinRef, m.MsgRef, m.Topic, m.Event, m.Payload})
}

func (d *Diff) JSON() ([]byte, error) {
	return json.Marshal(d)
}

func (d *Diff) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{d.JoinRef, d.MsgRef, d.Topic, d.Event, d.Payload})
}
