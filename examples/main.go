package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/canopyclimate/golive/changeset"
	"github.com/canopyclimate/golive/htmltmpl"
	"github.com/canopyclimate/golive/live"
	"github.com/dsnet/try"
	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	lr := r.NewRoute().Subrouter()

	// livemux := r.NewRoute().Subrouter()
	// set up liveview handlers, in a variety of styles to illustrate options
	lr.HandleFunc("/counter", func(w http.ResponseWriter, r *http.Request) {
		live.SetView(r, new(Counter))
	})
	lr.HandleFunc("/counter/{i}", func(w http.ResponseWriter, r *http.Request) {
		// Using a url component, with a helper
		x := try.E1(strconv.Atoi(mux.Vars(r)["i"]))
		c := live.MakeView[*Counter](r)
		c.Count = x
		live.SetView(r, c)
	})
	lr.HandleFunc("/nav", func(w http.ResponseWriter, r *http.Request) {
		live.SetView(r, new(Nav))
	})
	lr.HandleFunc("/more", func(w http.ResponseWriter, r *http.Request) {
		live.SetView(r, new(MoreEvents))
	})
	lr.HandleFunc("/photos", func(w http.ResponseWriter, r *http.Request) {
		live.SetView(r, new(MyPhotos))
	})
	lr.HandleFunc("/modal", func(w http.ResponseWriter, r *http.Request) {
		live.SetView(r, new(ModalDemo))
	})

	liveConfig := live.Config{
		Mux: lr,
		RenderLayout: func(w http.ResponseWriter, r *http.Request, lvd *live.LayoutDot) (any, *htmltmpl.Template) {
			lvd.PageTitle.Prefix = "GoLive - "
			return lvd, loadTemplate("./examples/layout.gohtml", myFuncs)
		},
	}
	// setup static route
	publicFS := http.FileServer(http.Dir("./public"))
	static := r.NewRoute().Subrouter()
	static.PathPrefix("/js/").Handler(publicFS)
	static.PathPrefix("/img/").Handler(publicFS)

	// setup "dead" route
	r.HandleFunc("/dead", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This is a dead route"))
	})

	// setup websocket route
	r.Handle("/live/websocket", live.NewWebsocketHandler(liveConfig))

	// use GoLive Middleware to make our LiveView mounts "live" and able to accept views
	r.Use(liveConfig.Middleware)

	// register mux router
	http.Handle("/", r)

	fmt.Printf("http://localhost:8080/counter\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// myFuncs is a example of custom funcs that can be used in templates.
var myFuncs = htmltmpl.FuncMap{
	"foo": func() string {
		return "foo"
	},
	"someTag": func() string {
		return "someTag"
	},
	"dict": func(values ...any) (map[string]any, error) {
		if len(values)%2 != 0 {
			return nil, errors.New("odd number of arguments to dict")
		}
		dict := make(map[string]any, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings, got %T for argument %d", values[i], i)
			}
			dict[key] = values[i+1]
		}
		return dict, nil
	},
}

func init() {
	// add live and changeset funcs to myFuncs
	for k, v := range live.Funcs() {
		myFuncs[k] = v
	}
	for k, v := range changeset.Funcs() {
		myFuncs[k] = v
	}
}

// loadTemplate loads a template from the given path, adding the provided FuncMaps.
func loadTemplate(path string, funcs htmltmpl.FuncMap) *htmltmpl.Template {
	name := filepath.Base(path)
	tmpl := htmltmpl.New(name).Funcs(funcs)
	return htmltmpl.Must(tmpl.ParseFiles(path))
}

// use a GoPlayground-based changeset to validate the form input
// and convert it to a struct
var ga = changeset.NewGoPlaygroundChangesetConfig()
var cc = changeset.Config{
	Validator: ga,
	Decoder:   ga,
}

// Person is the struct that we will validate using form input and changesets
type Person struct {
	First string `validate:"min=4"`
	Last  string `validate:"min=2"`
}

// Counter is a live.View that has a counter, a basic form, and a ticker.
// TODO split this into different views instead of the amalgamation it is now.
type Counter struct {
	Count       int
	Changeset   *changeset.Changeset[Person]
	First, Last string
	Ticks       int
	ticker      *time.Ticker
}

func (c *Counter) Mount(ctx context.Context, p live.Params) error {
	if c.Count == 0 {
		c.Count = 1
	}
	c.Changeset = changeset.New[Person](&cc, nil)
	c.Ticks = 10
	if c.ticker == nil {
		c.ticker = time.NewTicker(time.Second)
		go func() {
			for range c.ticker.C {
				live.SendInfo(ctx, &live.Info{Type: "tick"})
			}
		}()
	}
	return nil
}

func (c *Counter) HandleInfo(ctx context.Context, ci *live.Info) error {
	// Increment the ticker.
	c.Ticks++
	return nil
}

func (c *Counter) HandleEvent(ctx context.Context, e *live.Event) error {
	switch e.Type {
	// click events
	case "increment":
		c.Count++
	case "decrement":
		c.Count--
	// form events
	case "change":
		// validate input
		err := c.Changeset.Update(e.Data, e.Type)
		if err != nil {
			return err
		}
	case "submit":
		// validate input
		err := c.Changeset.Update(e.Data, e.Type)
		if err != nil {
			return err
		}
		// if valid "Save" the data
		if c.Changeset.Valid() {
			// "Save" the data
			p, err := c.Changeset.Struct()
			if err != nil {
				return err
			}
			c.First = p.First
			c.Last = p.Last
			// clear the changeset
			c.Changeset = changeset.New[Person](&cc, nil)
		}
	case "redirect":
		// redirect to the given path
		url, e := url.Parse("/dead")
		if e != nil {
			return e
		}
		live.Redirect(ctx, url)
	}
	return nil
}

func (c *Counter) Render(ctx context.Context, meta *live.Meta) (any, *htmltmpl.Template) {
	return c, htmltmpl.Must(htmltmpl.New("liveView").Funcs(myFuncs).Parse(`
			<div>
				Go to Nav: {{ liveNav "navigate" "/nav" (dict "" "") "Nav" }}
				<h1>Count is: {{ .Count }}</h1>
				<button phx-click="decrement">-</button>
				<button phx-click="increment">+</button>
			</div>
			{{ foo}}
			<form phx-submit="submit" phx-change="change">
				First {{ inputTag .Changeset "First" }}
				{{ errorTag .Changeset "First" }}
				<br />
				Last {{ inputTag .Changeset "Last" }}
				{{ errorTag .Changeset "Last" }}
				<br />
				<input type="submit" value="Submit" />
			</form>
			<button phx-click="redirect">Redirect</button>
			{{ if and .First .Last }}
				<h1>Hello {{ .First }} {{ .Last }}</h1>
			{{ end }}

			<div>
			  Counter that updates every second: {{ .Ticks }}
			</div>
		`))
}

func (c *Counter) Close() error {
	c.ticker.Stop()
	return nil
}

type Nav struct {
	Items map[string]string
	Item  string
}

func (n *Nav) Mount(ctx context.Context, p live.Params) error {
	// add a few different pieces of data to the context that
	// we will use for liveNav
	n.Items = map[string]string{
		"1": "Item 1",
		"2": "Item 2",
		"3": "Item 3",
	}
	return nil
}

func (n *Nav) HandleParams(ctx context.Context, url *url.URL) error {
	item := url.Query().Get("item")
	if item == "" {
		item = "1"
	}
	n.Item = n.Items[item]
	live.PageTitle(ctx, fmt.Sprintf("Item %s", n.Item))
	return nil
}

func (n *Nav) Render(ctx context.Context, meta *live.Meta) (any, *htmltmpl.Template) {
	return n, loadTemplate("./examples/nav.gohtml", myFuncs)
}

func (n *Nav) PageTitleConfig() live.PageTitleConfig {
	return live.PageTitleConfig{
		Title:  "NavView",
		Prefix: " Pre - ",
		Suffix: " - Suf",
	}
}

// Example with keyup, keydown, blur, focus
type MoreEvents struct {
	Volume  int
	Focused bool
}

func (v *MoreEvents) HandleEvent(ctx context.Context, e *live.Event) error {
	event := e.Type
	// if event was a key event, use the key name as the event
	if event == "key_update" {
		event = e.Data.Get("key")
	}
	switch event {
	case "up", "ArrowUp":
		v.Volume = int(math.Min(float64(v.Volume)+10.0, 100))
	case "down", "ArrowDown":
		v.Volume = int(math.Min(float64(v.Volume)-10.0, 100))
	case "focus":
		v.Focused = true
	case "blur":
		v.Focused = false
	}
	return nil
}

func (v *MoreEvents) Render(ctx context.Context, meta *live.Meta) (any, *htmltmpl.Template) {
	return v, htmltmpl.Must(htmltmpl.New("liveView").Funcs(myFuncs).Parse(`
			<div>
				<div>
					<h2>Volume: {{ .Volume}}</h2>
          <progress
            id="volume_control"
            style="width: 300px; height: 2em;"
            value="{{ .Volume }}"
            max="100"></progress>
					<br />
					<p>Use ⬇️ or ⬆️ keys to control the volume or buttons below</p>
        	<button phx-click="down" phx-window-keydown="key_update" phx-key="ArrowDown">⬇️</button>
          <button phx-click="up" phx-window-keydown="key_update" phx-key="ArrowUp">⬆️</button>
				</div>
				<div style="margin-top: 20px">
					<h2>Blur/Focus</h2>				
					<input type="text" name="Focused" placeholder="click to focus" phx-focus="focus" phx-blur="blur" />
					<br />
					Input is {{ if not .Focused}}not{{end}} focused
				</div>
			</div>
		`))
}

type Photo struct {
	ID   string
	Name string
	URL  string
}

type PhotosForm struct {
	Photos []Photo
}

type MyPhotos struct {
	PhotosForm PhotosForm
	Photos     []Photo
}

const uploadName = "photos"

func (lv *MyPhotos) Mount(ctx context.Context, p live.Params) error {
	live.AllowUpload(ctx, uploadName, live.UploadConstraints{
		Accept:      []string{".png", ".jpg", ".jpeg", ".gif"}, // only allow images
		MaxEntries:  3,                                         // only 3 entries per upload
		MaxFileSize: 5 * 1024 * 1024,                           // 5MB
	})
	return nil
}

func (lv *MyPhotos) Render(ctx context.Context, meta *live.Meta) (any, *htmltmpl.Template) {
	type myPhotos struct {
		*MyPhotos
		UploadPhotos *live.UploadConfig
	}
	dot := myPhotos{
		MyPhotos:     lv,
		UploadPhotos: meta.Uploads[uploadName],
	}
	return dot, loadTemplate("./examples/photos.gohtml", myFuncs)
}

func (lv *MyPhotos) HandleEvent(ctx context.Context, e *live.Event) error {

	switch e.Type {
	case "cancel":
		ref := e.Data.Get("ref")
		live.CancelUpload(ctx, uploadName, ref)
	case "save":
		// add completed files to the changeset
		completed, _ := live.UploadedEntries(ctx, uploadName)
		photos := make([]Photo, 0, len(completed))
		for _, entry := range completed {
			exts, _ := mime.ExtensionsByType(entry.Type)
			if len(exts) == 0 {
				exts = []string{".bin"}
			}

			loc := fmt.Sprintf("%s%s", entry.UUID, exts[0])

			// append the photos to the list
			photos = append(photos, Photo{
				ID:   entry.UUID,
				Name: entry.Name,
				URL:  filepath.Join(string(filepath.Separator), "img", loc),
			})
		}

		lv.Photos = append(lv.Photos, photos...)

		// "consume" a.k.a remove the uploaded entries from the upload config
		live.ConsumeUploadedEntries(ctx, uploadName, func(meta live.ConsumeUploadedEntriesMeta, entry live.UploadEntry) any {
			// we could create thumbnails, scan for viruses, etc.
			// but for now move the data from the temp file (meta.path) to a public directory
			input, err := os.ReadFile(meta.Path)
			if err != nil {
				panic(err)
			}

			exts, _ := mime.ExtensionsByType(entry.Type)
			if len(exts) == 0 {
				exts = []string{".bin"}
			}
			loc := fmt.Sprintf("%s%s", entry.UUID, exts[0])
			publicImg := filepath.Join(".", "public", "img")
			try.E(os.MkdirAll(publicImg, 0777))
			dest := filepath.Join(publicImg, loc)
			err = os.WriteFile(dest, input, 0644)
			if err != nil {
				panic(err)
			}
			return nil
		})

	}
	return nil
}

type ModalDemo struct {
	Text string
}

func (m *ModalDemo) ShowModal() *live.JS {
	js := &live.JS{}
	js.Show(&live.ShowOpts{To: "#modal"})
	js.Show(&live.ShowOpts{To: "#modal-content"})
	return js
}

func (m *ModalDemo) HideModal() *live.JS {
	js := &live.JS{}
	js.Hide(&live.HideOpts{To: "#modal"})
	js.Hide(&live.HideOpts{To: "#modal-content"})
	return js
}

func (m *ModalDemo) Toggle(target string) *live.JS {
	js := &live.JS{}
	js.Toggle(&live.ToggleOpts{To: target})
	return js
}

func (m *ModalDemo) Mount(ctx context.Context, p live.Params) error {
	m.Text = "Hello world!"
	return nil
}

func (m *ModalDemo) Render(ctx context.Context, meta *live.Meta) (any, *htmltmpl.Template) {
	return m, loadTemplate("./examples/modal.gohtml", myFuncs)
}
