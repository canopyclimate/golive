package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/canopyclimate/golive/htmltmpl"
	"github.com/canopyclimate/golive/internal/tmpl"
	"github.com/canopyclimate/golive/live/internal/phx"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Config is the configuration for a live application.
type Config struct {
	// LayoutTemplate is a template that wraps all Views
	LayoutTemplate *htmltmpl.Template
	// Mux is a http.Handler that routes requests to Views
	Mux http.Handler
	// PageTitleConfig is a configuration for the page title if the application
	// uses the liveTitleTag template function in its LayoutTemplate.
	PageTitleConfig PageTitleConfig
	// MakeCSRFToken, if non-nil, will be called when a View is requested and should return a valid CSRF token.
	// If nil, a UUID will be used instead.
	MakeCSRFToken func(*http.Request) string
}

type liveViewRequestContextKey struct{}
type liveViewContainer struct {
	lv View
}

func (c *Config) viewForRequest(w http.ResponseWriter, r *http.Request, currentView View) (View, int) {
	container := &liveViewContainer{
		lv: currentView,
	}
	r = r.WithContext(context.WithValue(r.Context(), liveViewRequestContextKey{}, container))

	rw := &joinHandler{
		w: w,
	}
	c.Mux.ServeHTTP(rw, r)
	return container.lv, rw.code
}

func (c *Config) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for cases where our config's router is its own Mux.
		// In that case we will re-enter the middleware and should no-op.
		_, ok := w.(*joinHandler)
		if ok {
			next.ServeHTTP(w, r)
			return
		}
		// Get the LiveView if one is routable.
		lv, code := c.viewForRequest(w, r, nil)

		// If the inner router 500s, cease the middleware chain.
		if code%100 == 5 {
			return
		}

		// If no view was found continue the chain without upgrading the request to a live one;
		// the outer router will presumably serve this route, but we no longer care about it.
		if lv == nil {
			next.ServeHTTP(w, r)
			return
		}

		// At this point we know this is a "live" route.
		// Configure things with our view and call the appropriate lifecycle methods.

		// if View implements HasPageTitleConfig interface then
		// use the config to set the page title
		ptc := c.PageTitleConfig
		if p, ok := lv.(PageTitleConfigurer); ok {
			ptc = p.PageTitleConfig()
		}

		// Run initial Lifecycle Mount => HandleParams => Render
		// We never call HandleEvent or HandleInfo for HTTP requests
		ctx := r.Context()
		// add a faux socket for uploadConfigs
		uploadConfigs := make(map[string]*UploadConfig)
		ctx = withSocket(ctx, &socket{
			uploadConfigs: uploadConfigs,
		})

		// if View implements Mounter interface then call Mount
		m, ok := lv.(Mounter)
		if ok {
			err := m.Mount(ctx, Params{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// if View implements ParamsHandler interface then call HandleParams
		hp, ok := lv.(ParamsHandler)
		if ok {
			err := hp.HandleParams(ctx, r.URL)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		var csrf string
		if c.MakeCSRFToken != nil {
			csrf = c.MakeCSRFToken(r)
		} else {
			csrf = uuid.New().String()
		}
		meta := Meta{
			Uploads:   uploadConfigs,
			CSRFToken: csrf,
		}

		t := lv.Render(ctx, meta)

		dot := LiveViewDot{
			LiveViewID:   uuid.New().String(), // TODO use nanoID or something shorter?
			CSRFToken:    csrf,
			View:         lv,
			PageTitle:    ptc,
			ViewTemplate: t,
			Meta:         meta,
		}

		err := c.LayoutTemplate.Execute(w, dot)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Meta is the metadata passed to a View's Render method as well as added to
// the template context via the .Meta field.
type Meta struct {
	CSRFToken string
	URL       url.URL
	Uploads   map[string]*UploadConfig
}

// Params is the data passed to a View's Mount method.
type Params struct {
	CSRFToken string
	Mounts    int
	Data      map[string]any
}

// View is a live view which requires a Render method in order to be
// rendered for HTML and WebSocket requests.
type View interface {
	Render(context.Context, Meta) *htmltmpl.Template
}

// Mounter is an interface that can be implemented by a View to be notified
// when it is mounted.
type Mounter interface {
	Mount(context.Context, Params) error
}

// ParamsHandler is an interface that can be implemented by a View to be notified
// when the URL parameters change (and after a view is first mounted).
type ParamsHandler interface {
	HandleParams(context.Context, *url.URL) error
}

// EventHandler is an interface that can be implemented by a View to be notified
// when user events are received from the client.
type EventHandler interface {
	HandleEvent(context.Context, *Event) error
}

// InfoHandler is an interface that can be implemented by a View to be notified
// when info (i.e. "internal") messages are received from the server.
type InfoHandler interface {
	HandleInfo(context.Context, *Info) error
}

// PageTitleConfgurer is an interface that can be implemented by a View to
// configure the liveTitleTag template function.
type PageTitleConfigurer interface {
	PageTitleConfig() PageTitleConfig
}

// A Router creates a Handler given a URL.
// TODO: rethink this with a better muxer, maybe the standard library muxer.
type Router = func(*url.URL) View

// NewWebsocketHandler returns a http.Handler that handles upgrading
// HTTP requests to WebSockets and handling message routing.
func NewWebsocketHandler(c Config) *WebsocketHandler {
	return &WebsocketHandler{config: c}
}

// WebsocketHandler handles Websocket requests and message routing.
type WebsocketHandler struct {
	config Config
}

func (x *WebsocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Potentially unwrap the joinHandler.
	// This can happen if the user is using the same router for live.Config.Mux
	// and for routing to the WebsocketHandler.
	j, ok := w.(*joinHandler)
	if ok {
		w = j.w
	}

	// TODO: route maps
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic("TODO: what? just disconnect?")
	}
	defer conn.Close() // TODO: is this right? probably...?
	s := &socket{
		req:           r,
		conn:          conn,
		config:        x.config,
		readerr:       make(chan error),
		msg:           make(chan *phx.Msg),       // TODO: buffered?
		info:          make(chan *Info),          // TODO: buffered?
		upload:        make(chan *phx.UploadMsg), // TODO: buffered?
		uploadConfigs: make(map[string]*UploadConfig),
	}
	go s.read()
	s.serve(r.Context())
}

func (s *socket) read() {
	for {
		msgType, msg, err := s.conn.ReadMessage()
		if err != nil {
			s.readerr <- fmt.Errorf("websocket read: %v", err)
			return
		}
		if msgType == websocket.BinaryMessage {
			um := &phx.UploadMsg{}
			err := um.UnmarshalBinary(msg)
			if err != nil {
				s.readerr <- fmt.Errorf("unmarshaling upload message: %v", err)
				return
			}
			s.upload <- um
			continue
		}

		pm, err := phx.Parse(msg)
		if err != nil {
			s.readerr <- fmt.Errorf("malformed phx message: %v", err)
			return
		}
		s.msg <- pm
	}
}

func (s *socket) serve(ctx context.Context) {
	ctx = withSocket(ctx, s)

	for {
		var r []byte
		res := [][]byte{}
		var err error
		select {
		case info := <-s.info:
			r, err = s.handleInfo(ctx, info)
			res = append(res, r)
		case pm := <-s.msg:
			r, err = s.dispatch(ctx, pm)
			res = append(res, r)
		case um := <-s.upload:
			res, err = s.handleUpload(ctx, um)
		case err := <-s.readerr:
			// TODO: what?
			fmt.Printf("websocket read failed: %v\n", err)
			return
		}
		if err != nil {
			fmt.Println("error:", err)
			panic("TODO: what?")
		}
		for _, m := range res {
			err = s.conn.WriteMessage(websocket.TextMessage, m)
			if err != nil {
				// TODO: what? the client has disconnected, so we should probably just hang up
				log.Println(err)
				return
			}
		}
	}
}

// A socket tracks an individual websocket connection.
type socket struct {
	req               *http.Request // http request that initiated this websocket connection
	conn              *websocket.Conn
	config            Config
	view              View
	id                string
	msg               chan *phx.Msg
	info              chan *Info
	upload            chan *phx.UploadMsg
	readerr           chan error
	title             string
	url               url.URL
	csrfToken         string
	uploadConfigs     map[string]*UploadConfig
	activeUploadRef   string
	activeUploadTopic string
}

func (s *socket) dispatch(ctx context.Context, msg *phx.Msg) ([]byte, error) {
	event := msg.Event
	switch event {
	case "phx_join":
		// check if topic starts with "lv:" or "lvu:"
		// "lv:" is a liveview
		// "lvu:" is a liveview upload
		switch {
		case strings.HasPrefix(msg.Topic, "lv:"):

			// first we need to read the msg to see what route we're on
			// on join the message payload should include a "url" key or
			// a "redirect" key
			urlStr := ""
			if u, ok := msg.Payload["url"].(string); ok {
				urlStr = u
			} else if u, ok := msg.Payload["redirect"].(string); ok {
				urlStr = u
			}
			if urlStr == "" {
				return nil, fmt.Errorf("no url or redirect found in payload")
			}
			// parse url
			url, err := url.Parse(urlStr)
			if err != nil {
				return nil, fmt.Errorf("could not parse url: %v", err)
			}
			// look up View by url path
			// TODO: we could thread the http request that initiated this request
			// all the way through to here and re-use it except for the newly
			// requested URL, which might be useful for routing if it contains
			// headers or the like(?)
			r := s.req.Clone(s.req.Context()) // todo: background context?
			r.URL = url
			var code int
			s.view, code = s.config.viewForRequest(nil, r, nil)
			if code%100 == 5 {
				return nil, fmt.Errorf("Error finding view for url: %v", err)
			}
			if s.view == nil {
				// TODO: something better here!
				return nil, fmt.Errorf("404")
			}

			// get data from params
			rawParams, ok := msg.Payload["params"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("params not found in payload")
			}
			// pull out known params
			params := Params{
				CSRFToken: rawParams["_csrf_token"].(string),
				Mounts:    int(rawParams["_mounts"].(float64)),
				Data:      rawParams,
			}

			s.id = msg.Topic
			s.url = *url
			s.csrfToken = params.CSRFToken

			// Join is the initalize event and the only time we call Mount on the view.
			// Only call Mount if the view implements Mounter
			m, ok := s.view.(Mounter)
			if ok {
				err := m.Mount(ctx, params)
				if err != nil {
					return nil, err
				}
			}
			// Also call HandleParams during join to give the LiveView a chance
			// to update its state based on the URL params
			hp, ok := s.view.(ParamsHandler)
			if ok {
				err := hp.HandleParams(ctx, url)
				if err != nil {
					return nil, err
				}
			}

			t, err := s.renderToTree(ctx)
			if err != nil {
				return nil, err
			}
			json, err := t.JSON()
			if err != nil {
				return nil, err
			}
			return phx.NewRendered(*msg, json).JSON()
		case strings.HasPrefix(msg.Topic, "lvu:"):
			// set active upload topic
			s.activeUploadTopic = msg.Topic
			// basically send back an ack
			return phx.NewEmptyReply(*msg).JSON()
		default: // unknown phx_join topic
			return nil, fmt.Errorf("unknown join topic: %q", msg.Topic)
		}
	case "heartbeat":
		// TODO - set a timer that gets reset on every heartbeat
		// if the timer expires, we should Close the live view
		// and try to send a message to the client saying we are closing
		// the connection
		return phx.NewHeartbeat(msg.MsgRef).JSON()
	case "event":
		// all events payloads have a few shared keys
		et := msg.Payload["type"].(string)
		ee := msg.Payload["event"].(string)
		// cid := phxMsg.Payload["cid"] // component ID (not used yet)

		var t *tmpl.Tree
		switch et {
		case "click", "keyup", "keydown", "blur", "focus", "hook":
			// payload should be map[string]any
			v := msg.Payload["value"].(map[string]any)
			// convert the value map to a url.Values
			vals := url.Values{}
			for k, v := range v {
				// Convert v to strings
				// TODO this should work for numbers, bools, strings
				// but it doesn't work for arrays or objects
				// TBH - I am not sure we'll ever get those types here
				vals.Add(k, fmt.Sprint(v))
			}
			// check if the click is a lv:clear-flash event
			// which does not invoke HandleEvent but should
			// set the flash value to "" and send a responseDiff
			if event == "lv:clear-flash" {
				flashKey := vals.Get("key")
				// TODO clear flash
				// s.handler.ClearFlash(flashKey)
				fmt.Printf("clear flash event: %s\n", flashKey)
			} else {
				eh, ok := s.view.(EventHandler)
				if !ok {
					return nil, fmt.Errorf("view %T does not implement EventHandler", s.view)
				}
				err := eh.HandleEvent(ctx, &Event{Type: ee, Data: vals})
				if err != nil {
					return nil, err
				}
			}
		case "form":
			vals, err := url.ParseQuery(msg.Payload["value"].(string))
			if err != nil {
				return nil, err
			}
			eh, ok := s.view.(EventHandler)
			if !ok {
				return nil, fmt.Errorf("view %T does not implement EventHandler", s.view)
			}
			err = eh.HandleEvent(ctx, &Event{Type: ee, Data: vals})
			if err != nil {
				return nil, err
			}

			if uploads, ok := msg.Payload["uploads"].(map[string]any); ok && len(uploads) != 0 {
				// get _target from form data
				uc_target := vals.Get("_target")
				// get the upload config from the uploadConfigs map
				uc := s.uploadConfigs[uc_target]
				// found the upload config & uploads reference the upload config
				if uc != nil && uc.Ref != "" && uploads[uc.Ref] != nil {
					uc.AddEntries(uploads[uc.Ref].([]any))
				}
			}

		default:
			return nil, fmt.Errorf("unknown event type: %v", et)
		}

		// Now re-render the Tree
		t, err := s.renderToTree(ctx)
		if err != nil {
			return nil, err
		}
		// TODO diff the Tree / Context and send only the changes
		// for now, we send it all back...
		// Note, we return a "diff" instead of a "rendered" response
		diff, err := t.JSON()
		if err != nil {
			return nil, err
		}
		return phx.NewReplyDiff(*msg, diff).JSON()
	case "live_patch":
		r := s.req.Clone(s.req.Context()) // todo: background context?
		v, code := s.config.viewForRequest(nil, r, s.view)
		s.view = v // update the view
		if code%100 == 5 {
			return nil, fmt.Errorf("Status code 500 patching LiveView in %v", r.URL)
		}
		url, err := url.Parse(msg.Payload["url"].(string))
		if err != nil {
			return nil, err
		}
		hp, ok := s.view.(ParamsHandler)
		if ok {
			err := hp.HandleParams(ctx, url)
			if err != nil {
				return nil, err
			}
		}
		// Now re-render the Tree
		lt, err := s.renderToTree(ctx)
		if err != nil {
			return nil, err
		}
		// TODO diff the Tree / Context and send only the changes
		// for now, we send it all back...
		// Note, we return a "diff" instead of a "rendered" response
		diff, err := lt.JSON()
		if err != nil {
			return nil, err
		}
		return phx.NewReplyDiff(*msg, diff).JSON()
	case "phx_leave":
		if s.view != nil {
			// check if the view implements the Closer interface
			// and call Close() if it does. it is not an error if
			// the view does not implement the Closer interface.
			c, ok := s.view.(io.Closer)
			if ok {
				err := c.Close()
				return nil, err
			}
		}
		return nil, nil
	case "allow_upload":
		// re-render tree
		lt, err := s.renderToTree(ctx)
		if err != nil {
			return nil, err
		}

		// get upload ref and entries from payload
		ref := msg.Payload["ref"].(string)
		s.activeUploadRef = ref
		entries := msg.Payload["entries"].([]any)

		// get upload config from uploadConfigs map
		var uc *UploadConfig
		for _, u := range s.uploadConfigs {
			if u.Ref == ref {
				uc = u
				break
			}
		}
		if uc == nil {
			return nil, fmt.Errorf("no upload config found for ref: %s", ref)
		}

		constraints := NewUploadConstraints(uc)

		// echo back the ref and entries to the client
		entriesMap := make(map[string]any)
		entriesMap[ref] = ref
		for _, entry := range entries {
			entriesMap[entry.(map[string]any)["ref"].(string)] = entry
		}

		// build the diff component JSON
		diffJson, err := lt.JSON()
		if err != nil {
			return nil, err
		}
		configJson, err := json.Marshal(constraints)
		if err != nil {
			return nil, err
		}
		entriesJson, err := json.Marshal(entries)
		if err != nil {
			return nil, err
		}

		return phx.NewUploadReplyDiff(*msg, diffJson, configJson, entriesJson).JSON()
	case "progress":
		ref := msg.Payload["ref"].(string)
		entryRef := msg.Payload["entry_ref"].(string)
		progress := int(msg.Payload["progress"].(float64))

		// get the upload config from the uploadConfigs map
		var uc *UploadConfig
		for _, u := range s.uploadConfigs {
			if u.Ref == ref {
				uc = u
				break
			}
		}
		if uc == nil {
			return nil, fmt.Errorf("no upload config found for ref: %s", ref)
		}
		// find the entry in the upload config
		for i, entry := range uc.Entries {
			if entry.Ref == entryRef {
				uc.Entries[i].Progress = progress
				uc.Entries[i].Done = progress == 100
				break
			}
		}

		// re-render tree
		// Now re-render the Tree
		lt, err := s.renderToTree(ctx)
		if err != nil {
			return nil, err
		}
		// TODO diff the Tree / Context and send only the changes
		// for now, we send it all back...
		// Note, we return a "diff" instead of a "rendered" response
		diff, err := lt.JSON()
		if err != nil {
			return nil, err
		}
		return phx.NewReplyDiff(*msg, diff).JSON()
	}
	return nil, fmt.Errorf("unknown event: %s", event)

}

// handleInfo receives internal messages then runs: HandleInfo => Render
// on the View before sending a diff back to the client
func (s *socket) handleInfo(ctx context.Context, info *Info) ([]byte, error) {

	ih, ok := s.view.(InfoHandler)
	if !ok {
		return nil, fmt.Errorf("view does not implement InfoHandler")
	}
	err := ih.HandleInfo(ctx, info)
	if err != nil {
		return nil, err
	}
	t, err := s.renderToTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering error: %v", err)
	}
	diff, err := t.JSON()
	if err != nil {
		return nil, err
	}
	return phx.NewDiff(nil, s.id, diff).JSON()
}

func (s *socket) handleUpload(ctx context.Context, up *phx.UploadMsg) (res [][]byte, err error) {
	// get ref from topic
	ref := strings.Split(up.Topic, ":")[1]

	// get uploadConfig for activeUploadRef
	var uc *UploadConfig
	for _, v := range s.uploadConfigs {
		if v.Ref == s.activeUploadRef {
			uc = v
			break
		}
	}
	if uc == nil {
		return res, fmt.Errorf("no upload config found for ref: %s", s.activeUploadRef)
	}

	// get the upload entry by ref
	var firstUpload bool
	var entry *UploadEntry
	for _, e := range uc.Entries {
		if e.Ref == ref {
			entry = &e
			// if this is first upload (e.g. progress == 0) then we send a empty diff to the client
			firstUpload = e.Progress == 0
			break
		}
	}
	if entry == nil {
		return res, fmt.Errorf("no upload entry found for ref: %s", ref)
	}

	// save the data to a temp file
	tdir := filepath.Join(os.TempDir(), fmt.Sprintf("golive-%s", s.activeUploadRef))
	err = os.MkdirAll(tdir, 0777)
	if err != nil {
		return res, err
	}
	file, err := os.OpenFile(filepath.Join(tdir, entry.UUID), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return res, err
	}
	defer file.Close()
	_, err = file.Write(up.Payload)

	if firstUpload {
		d, err := phx.NewDiff(&up.JoinRef, s.id, []byte("{}")).JSON()
		if err != nil {
			return res, err
		}
		res = append(res, d)
	}

	// append lvu response
	lvuRes, err := phx.NewEmptyUploadReply(*up).JSON()
	if err != nil {
		return res, err
	}
	res = append(res, lvuRes)

	return res, nil
}

var upgrader = &websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

func (s *socket) renderToTree(ctx context.Context) (*tmpl.Tree, error) {
	meta := Meta{
		URL:       s.url,
		CSRFToken: s.csrfToken,
		Uploads:   s.uploadConfigs,
	}
	t := s.view.Render(ctx, meta)

	dot, err := dotFromView(s.view)
	if err != nil {
		return nil, err
	}
	dot["Meta"] = meta

	tree, err := t.ExecuteTree(dot)
	// add title part to tree if it is set
	if s.title != "" {
		tree.Title = s.title
		s.title = ""
	}
	return tree, err
}

// Event is the event data sent from the client
type Event struct {
	Type string
	Data url.Values
}

// Info is internal event data from the server
type Info Event

// NewHTTPHandler returns a new HTTPHandler with the given config
func NewHTTPHandler(c Config) *HTTPHandler {
	return &HTTPHandler{config: c}
}

// HTTPHandler handles HTTP requests for a View
type HTTPHandler struct {
	config Config
}

// SendInfo sends an internal event to the View if it is connected to a WebSocket
func SendInfo(ctx context.Context, info *Info) {
	s := socketValue(ctx)
	if s == nil {
		return
	}
	s.info <- info
}

// PageTitle updates the page title for the View
func PageTitle(ctx context.Context, newTitle string) {
	s := socketValue(ctx)
	if s == nil {
		return
	}
	s.title = newTitle
}

type socketContextKey struct{}

// withSocket returns a context built by associating s with ctx.
func withSocket(ctx context.Context, s *socket) context.Context {
	return context.WithValue(ctx, socketContextKey{}, s)
}

// socketValue returns the socket, if any, associated with ctx.
func socketValue(ctx context.Context) *socket {
	s, _ := ctx.Value(socketContextKey{}).(*socket)
	return s
}

// dotFromView extracts the fields from the view and returns a map
// which is used as the basis for the dot used in the template
func dotFromView(lv View) (map[string]any, error) {
	// map struct fields to dot
	dot := make(map[string]any)
	v := reflect.ValueOf(lv)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	// ensure view is a struct
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("s.view should be a struct but is a %T", v)
	}
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := typ.Field(i)
		vf := v.Field(i)
		// skip unexported fields
		if vf.CanInterface() {
			dot[f.Name] = v.Field(i).Interface()
		}
	}
	return dot, nil
}

type PageTitleConfig struct {
	Title  string
	Prefix string
	Suffix string
}

type LiveViewDot struct {
	PageTitle    PageTitleConfig
	Static       string
	LiveViewID   string
	CSRFToken    string
	ViewTemplate *htmltmpl.Template
	View         View
	Meta         Meta
}
