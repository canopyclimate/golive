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
	"strings"

	"github.com/canopyclimate/golive/htmltmpl"
	"github.com/canopyclimate/golive/internal/tmpl"
	"github.com/canopyclimate/golive/live/internal/phx"
	"golang.org/x/time/rate"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Config is the configuration for a live application.
type Config struct {
	// Mux is a http.Handler that routes requests to Views.
	Mux http.Handler
	// ShouldHandleRequest indicates whether this Config should handle r.
	// If nil, it is assumed to return true.
	// This can be helpful when live views co-exist with non-live views.
	ShouldHandleRequest func(r *http.Request) bool
	// RenderLayout is a func that provides a way to render your base layout HTML for all LiveViews.
	// You are provides with the http.ResponseWriter, *http.Request, and a *LayoutDot and are responsible for
	// providing the "dot" and the template that will be executed on the initial HTML render.
	// Your template should always do at least the following:
	//  - Load your LiveView Client Javascript (e.g. <script defer type="text/javascript" src="/js/index.js"></script>) without this, your LiveView will not work.
	//  - Pass the LayoutDot to the liveViewContainerTag (i.e. {{ liveViewContainerTag .LayoutDot }})
	//  - Set the CSRF token in a meta tag (i.e. <meta name="csrf-token" content="{{ .LayoutDot.CSRFToken }}">)
	RenderLayout func(http.ResponseWriter, *http.Request, *LayoutDot) (any, *htmltmpl.Template)
	// OnViewError is called when an error occurs during a View lifecycle method (e.g. HandleEvent, HandleInfo, etc)
	// AND the view is connected to a socket (as opposed to the initial HTTP request). OnViewError may be nil
	// in which case the error is not logged or handled in any way.  Regardless of whether OnViewError is non-nil,
	// the javascript client will receive a "phx_error" message via the connected socket which in the case
	// of `HandleEvent` and `HandleInfo` will result in the client attempting to re-join the View.  For `HandleParams`,
	// the error will result in a page reload which will start the HTTP request lifecycle over again.
	OnViewError func(ctx context.Context, v View, url *url.URL, err error)
}

type (
	liveViewRequestContextKey struct{}
	liveViewContainer         struct {
		lv View
		r  *http.Request
	}
)

func (c *Config) viewForRequest(w http.ResponseWriter, r *http.Request, currentView View) (View, int, *http.Request) {
	container := &liveViewContainer{
		lv: currentView,
	}
	r = r.WithContext(context.WithValue(r.Context(), liveViewRequestContextKey{}, container))

	rw := &joinHandler{
		w: w,
	}
	c.Mux.ServeHTTP(rw, r)
	if container.r != nil {
		r = container.r
	}
	return container.lv, rw.code, r
}

func (c *Config) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.ShouldHandleRequest != nil && !c.ShouldHandleRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Get the LiveView if one is routable.
		lv, code, r := c.viewForRequest(w, r, nil)

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
		var ptc PageTitleConfig
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

		// Users may overwrite this in WriteLayout if they wish.
		csrf := uuid.New().String()
		meta := &Meta{
			Uploads:   uploadConfigs,
			CSRFToken: csrf,
		}

		lvd, lvt := lv.Render(ctx, meta)

		ldot := &LayoutDot{
			LiveViewID:   uuid.New().String(), // TODO use nanoID or something shorter?
			CSRFToken:    csrf,
			PageTitle:    ptc,
			viewTemplate: lvt,
			viewDot:      lvd,
		}

		// TODO: Fallback to a hardcoded base layout if WriteLayout isn't set.
		ld, lt := c.RenderLayout(w, r, ldot)
		err := lt.Execute(w, ld)
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
	// Render returns the dot and template needed to turn a LiveView into HTML.
	// Commonly the returned dot is the receiver; however, any data needed to render the template is acceptable.
	// Note that if the View uses any upload live.Funcs() the *Meta argument should be passed through to the template.
	Render(context.Context, *Meta) (any, *htmltmpl.Template)
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
	// TODO: route maps
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket for %T: %v", w, err)
		return
	}
	defer conn.Close() // TODO: is this right? probably...?
	s := &socket{
		req:            r,
		conn:           conn,
		config:         x.config,
		readerr:        make(chan error),
		msg:            make(chan *phx.Msg),
		info:           make(chan *Info),
		upload:         make(chan *phx.UploadMsg),
		nav:            make(chan *phx.Nav),
		uploadConfigs:  make(map[string]*UploadConfig),
		errTokenBucket: rate.NewLimiter(rate.Limit(1/15.0), 3), // at most one event per 15s on average, but 3 initial retries free
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
			if err == nil {
				res = append(res, r)
			}
		case pm := <-s.msg:
			r, err = s.dispatch(ctx, pm)
			if err == nil {
				res = append(res, r)
			}
		case um := <-s.upload:
			res, err = s.handleUpload(ctx, um)
		case nm := <-s.nav:
			r, err = nm.JSON()
			if err == nil {
				res = append(res, r)
			}
		case err := <-s.readerr:
			// String matching. Much sadness.
			if !strings.Contains(err.Error(), "websocket: close") {
				log.Printf("websocket read failed: %v", err)
			}
			return
		}
		if err != nil {
			// call configured error handler if set
			if s.config.OnViewError != nil {
				s.config.OnViewError(ctx, s.view, &s.url, err)
			}
			// Rate limit error responses. This prevents retries from overwhelming the server.
			// It would be better for the client to have some kind of graceful backoff,
			// but we don't control the client.
			s.errTokenBucket.Wait(ctx)
			// send "phx_error" message to client
			b, err := json.Marshal(phx.NewError(s.joinRef, s.msgRef, s.id))
			if err != nil {
				panic(err) // theoretically should never happen
			}
			res = append(res, b)
		}
		for _, m := range res {
			err = s.conn.WriteMessage(websocket.TextMessage, m)
			if err != nil {
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
	id                string // aka join topic
	joinRef           string // initial join ref
	msgRef            string // initial message ref
	msg               chan *phx.Msg
	info              chan *Info
	upload            chan *phx.UploadMsg
	nav               chan *phx.Nav
	events            []*Event
	readerr           chan error
	title             string
	url               url.URL
	redirect          string
	csrfToken         string
	uploadConfigs     map[string]*UploadConfig
	activeUploadRef   string
	activeUploadTopic string
	errTokenBucket    *rate.Limiter
	oldTree           *tmpl.Tree
}

func (s *socket) treeDiff(newTree *tmpl.Tree) []byte {
	oldTree := s.oldTree
	s.oldTree = newTree
	t := newTree
	if oldTree != nil {
		json := tmpl.DiffJSON(oldTree, newTree)
		fmt.Printf("old: %s\n\n\n new: %s\n\n\n diff: %s\n\n\n", oldTree.JSON(), newTree.JSON(), json)
		return json
	}
	return t.JSON()
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
			s.view, code, _ = s.config.viewForRequest(nil, r, nil)
			if code%100 == 5 {
				return nil, fmt.Errorf("error finding view for url: %v", err)
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
			s.joinRef = msg.JoinRef
			s.msgRef = msg.MsgRef
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
			return phx.NewRendered(*msg, s.treeDiff(t)).JSON()
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
				log.Printf("clear flash event: %s", flashKey)
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

			// handle uploads before calling HandleEvent
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

			// call the view's HandleEvent method if it implements EventHandler
			eh, ok := s.view.(EventHandler)
			if !ok {
				return nil, fmt.Errorf("view %T does not implement EventHandler", s.view)
			}
			err = eh.HandleEvent(ctx, &Event{Type: ee, Data: vals})
			if err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("unknown event type: %v", et)
		}
		// check if we have a redirect
		if s.redirect != "" {
			return phx.NewRedirect(*msg, s.redirect).JSON()
		}

		// Now re-render the Tree
		t, err := s.renderToTree(ctx)
		if err != nil {
			return nil, err
		}
		return phx.NewReplyDiff(*msg, s.treeDiff(t)).JSON()
	case "live_patch":
		r := s.req.Clone(s.req.Context()) // todo: background context?
		v, code, _ := s.config.viewForRequest(nil, r, s.view)
		s.view = v // update the view
		if code%100 == 5 {
			return nil, fmt.Errorf("status code 500 patching LiveView in %v", r.URL)
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
		return phx.NewReplyDiff(*msg, s.treeDiff(lt)).JSON()
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
		entriesJson, err := json.Marshal(entriesMap)
		if err != nil {
			return nil, err
		}

		// build the diff component JSON
		diffJson := s.treeDiff(lt)
		configJson, err := json.Marshal(constraints)
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
		return phx.NewReplyDiff(*msg, s.treeDiff(lt)).JSON()
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
	diff := t.JSON()
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
	err = os.MkdirAll(tdir, 0o777)
	if err != nil {
		return res, err
	}
	file, err := os.OpenFile(filepath.Join(tdir, entry.UUID), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
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
	meta := &Meta{
		URL:       s.url,
		CSRFToken: s.csrfToken,
		Uploads:   s.uploadConfigs,
	}
	dot, t := s.view.Render(ctx, meta)

	tree, err := t.ExecuteTree(dot)
	// add title part to tree if it is set
	if s.title != "" {
		tree.Title = s.title
		s.title = ""
	}
	// add events to tree if there are any
	if len(s.events) > 0 {
		for _, e := range s.events {
			rawJson, err := e.MarshalJSON()
			if err != nil {
				return nil, err
			}
			tree.Events = append(tree.Events, rawJson)
		}
		s.events = nil
	}
	return tree, err
}

// Event is the event data sent from the client
type Event struct {
	Type string
	Data url.Values
}

// MarshalJSON implements json.Marshaler for Event
func (e *Event) MarshalJSON() ([]byte, error) {
	// if a key is multi-valued, send it as an array
	// otherwise send it as a single value
	vals := make(map[string]any)
	for k, v := range e.Data {
		if len(v) == 1 {
			vals[k] = v[0]
		} else {
			vals[k] = v
		}
	}
	return json.Marshal([]any{
		e.Type,
		vals,
	})
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
	// TODO should we do this in a goroutine?
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

type LiveNavType string

const (
	NavPatch    LiveNavType = "live_patch"
	NavRedirect LiveNavType = "live_redirect"
)

// PushNav supports push patching and push redirecting from server to View
func PushNav(ctx context.Context, typ LiveNavType, path string, params url.Values, replaceHistory bool) error {
	s := socketValue(ctx)
	if s == nil {
		return nil
	}
	// build new URL from existing URL and new path and params
	url := url.URL{Path: path, RawQuery: params.Encode()}
	to := s.url.ResolveReference(&url)

	kind := "push"
	if replaceHistory {
		kind = "replace"
	}

	// call HandleParams if view implements ParamsHandler
	hp, ok := s.view.(ParamsHandler)
	if ok {
		err := hp.HandleParams(ctx, to)
		if err != nil {
			return err
		}
	}

	// send nav event to view
	p := phx.NavPayload{Kind: kind, To: to.String()}
	nm := phx.NewNav(s.id, string(typ), p)

	// don't block waiting for nav channel to be read
	go func() { s.nav <- nm }()
	return nil
}

// Redirect sends an event to the View that triggers a full page load to url.
func Redirect(ctx context.Context, url *url.URL) error {
	s := socketValue(ctx)
	if s == nil {
		return nil
	}
	s.redirect = url.String()
	return nil
}

// PushEvent sends an event to the View which a Hook can respond to
func PushEvent(ctx context.Context, e Event) error {
	s := socketValue(ctx)
	if s == nil {
		return nil
	}
	// queue event to be sent to view
	s.events = append(s.events, &e)
	return nil
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

// PageTitleConfig structures the contents of a page’s title tag. It’s available in WriteLayout for your own use,
// and subsequent PageTitle() calls will update the title while preserving Prefix and Suffix.
type PageTitleConfig struct {
	Title  string
	Prefix string
	Suffix string
}

// LayoutDot is the information available when initially writing our your container layout.
// It should be passed into the liveViewContainerTag funcmap func if you choose to use it.
type LayoutDot struct {
	PageTitle    PageTitleConfig
	Static       string
	LiveViewID   string
	CSRFToken    string
	viewTemplate *htmltmpl.Template
	viewDot      any
}

func (d *LayoutDot) ExecuteViewTemplate(buf *strings.Builder) error {
	return d.viewTemplate.ExecuteTemplate(buf, d.viewTemplate.Name(), d.viewDot)
}
