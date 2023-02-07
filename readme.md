# GoLive

> **Warning**  
> GoLive is very much **alpha software**. We are just starting to use it in production at [Canopy](https://canopyclimate.com), and are confident that many of the assumptions we made in its initial design will be invalidated in the cold, hard light of reality. The surface API may change under you; however, the core ideas are here to stay.

GoLive is a library for building LiveViews in Go. The LiveView pattern, [as popularized in Elixir’s Phoenix framework](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html), shifts your UI’s state management and event handling to the server, calculating minimal diffs¹ to drive updates in your HTML over WebSockets.

While we encourage reading the first few paragraphs of the linked Elixir docs as that library is mature and well-documented, here is a short list of advantages to this pattern:

1. Because logic shifts to the server you are no longer reasoning about your web app in terms of a backend, frontend, and connective API tissue: it’s all backend. This means, for example, that it’s perfectly valid to query your database while handling button clicks—no request handling, hooks, or other indirection needed.
2. GoLive (like all flavors of LiveView) includes API for triggering state changes from elsewhere in your server. This allows for live-updating views driven by, say, a Kafka queue, using the exact same API as the rest of your UI and with no additional boilerplate.
3. The server will initially render a static version of your UI, serve it, and then _mount_ that UI by connecting to it via WebSocket. This means initial page loads are very fast.
4. Many LiveView apps involve little to no JavaScript; however, if you so desire you can always use the [escape hatches](https://hexdocs.pm/phoenix_live_view/js-interop.html) that the LiveView JavaScript client provides—we use the same client as Phoenix.

It’s also worth saying what GoLive is not: unlike Phoenix, GoLive does not endeavor to encompass everything you might need to build a web app. We have tried to hew to Go’s philosophy of smaller component libraries over huge monolithic frameworks. GoLive lets you bring your own `http.Handler`, styling libraries, intra-app communication patterns, etc.

That said, if a pattern or boilerplate commonly emerges in the use of GoLive, we may, with careful consideration, incorporate it to increase ease of use.

¹So we’re not actually diffing the results of state changes yet; we’re just sending down entire documents for now. We plan to [fix this](https://github.com/canopyclimate/golive/issues/1) without any change to surface API. Again—alpha software!

## Examples

In GoLive, LiveViews are structs that implement a few basic methods. Here’s a simple example of a counter that can be incremented and decremented:

```go
import (
    "context"

    "github.com/canopyclimate/golive/htmltmpl"
    "github.com/canopyclimate/golive/live"
)

type Counter struct {
    Count int
}

func (c *Counter) HandleEvent(ctx context.Context, e *live.Event) error {
    switch e.Type {
    case "increment":
        c.Count++
    case "decrement":
        c.Count--
    }
    return nil
}

func (c *Counter) Render(ctx context.Context, meta *live.Meta) *htmltmpl.Template {
    return htmltmpl.Must(htmltmpl.New("liveView").Parse(`
        <div>
            <h1>Count is: {{ .V.Count }}</h1>
            <button phx-click="decrement">-</button>
            <button phx-click="increment">+</button>
        </div>
    `))
}
```

As you can see, the struct itself represents the state of your view. The `phx-click` attributes correspond to event types in our `HandleEvent` handler. After an event is handled, the view is recalculated and new state communicated via WebSocket to the client where it is displayed. You can access your view’s fields and methods from your template as `.V`.

You can find more examples in the `examples/` directory. To run:

```
go run ./examples/main.go
```

## Installing

On your server:

```
go get github.com/canopyclimate/golive@latest
```

In your frontend:

```
npm i --save phoenix
npm i --save phoenx_live_view
```

And then add the following to whatever JavaScript you’ll be loading in your frontend (you probably want to load this file in whatever template your `live.Config.WriteLayout` writes out):

```js
import { Socket } from "phoenix";
import { LiveSocket } from "phoenix_live_view";

let csrfToken = document
  .querySelector("meta[name='csrf-token']")
  ?.getAttribute("content");
// "/live" below should be whatever path you set up for your WebSocket.
let liveSocket = new LiveSocket("/live", Socket, {
  params: { _csrf_token: csrfToken },
});

// connect if there are any LiveViews on the page
liveSocket.connect();
```

## About `htmltmpl`

This package includes a fork of the standard library’s `html/template` package named `htmltmpl`. It has similar security guarantees to the original package. The fork exists to support the diffing needed to track changes in your HTML based on the live view’s state.

## Routing & Config

As noted above, GoLive does not endeavor to own your app’s routing. Instead, we provide a `net/http` middleware which you then use—any compatible routing library will do.

By using a middleware we can reuse your existing request handling stack while checking if a given handler has set a LiveView. If not, we fall back to your existing routing: in middleware terms, we simply call the next middleware in the stack and return. If so, we're able to hijack the request stack to write out your unmounted, rendered LiveView, and then mount your LiveView when it connects via WebSocket.

In order to route to your LiveViews you will need to create a `live.Config`. This struct has a `Mux` field, an `http.Handler` that maps routes to the handlers that will create your LiveViews (more on those in a bit). Typically, this `Mux` will be a subrouter within your app; if an incoming request maps to a “live” route it will be through `Mux`, and if `Mux` 404s or otherwise does not handle a request it will fall through to the rest of your app. Here’s what that looks like in practice:

```go
// Using gorilla/mux as an example, but this can be any compatible router.
r := mux.NewRouter()

liveRouter := mux.NewRoute().Subrouter()

// Some funcmap you’d like to use in your root layout.
// Remember to merge in live.Funcs() and changeset.Funcs()
// if you’re interested in using them.
var funcs template.FuncMap
tmpl := htmltmpl.New("layout.gohtml").Funcs(funcs)
t := htmltmpl.Must(tmpl.ParseFiles("path/to/layout.gohtml"))

liveConfig := live.Config{
    Mux:         liveRouter,
    // This hook lets us inject whatever data/HTML we might need to our live views.
    WriteLayout: func(w http.ResponseWriter, r *http.Request, lvd *live.LayoutDot) error {
        lvd.PageTitle.Prefix = "GoLive - "
        dot := make(map[string]any)
        dot["LiveView"] = lvd
        return t.Execute(w, dot)
    },
}

// Configure incoming requests to allow upgrading to LiveView.
// Note that you cannot use the middleware on the same router as your live.Config.Mux.
r.Use(liveConfig.Middleware)

// Handle WebSocket connections from mounted LiveViews.
r.Handle("/live/websocket", live.NewWebsocketHandler(liveConfig))

// Route to a LiveView, for example.
liveRouter.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
    live.SetView(r, new(Dashboard))
})

// Route to a LiveView that can be patched via path variables.
liveRouter.HandleFunc("/user/{user_id}", func(w http.ResponseWriter, r *http.Request) {
    x := try.E1(strconv.Atoi(mux.Vars(r)["user_id"]))
    p := live.MakeView[*UserProfile](r)
    p.UserID = x
    live.SetView(r, p)
})

// More routes could follow. Non-LiveView routes will work as expected.
```

> **Note**  
> When you patch a view in GoLive, we first give you an opportunity to re-handle the “request,” parsing it as needed, before calling `HandleParams`. In Phoenix terms, path params are handled different from URL query params: path params are parsed out at the muxer layer, URL query params in the more traditional `HandleParams` callback. This is a consequence of our decision to let you bring your own muxer, but may be unexpected for those familiar with Phoenix.

## live.JS

GoLive includes a struct, `JS`, that provides API to precompose client-side commands that do not require a roundtrip to the server, [much like Phoenix.LiveView.JS](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.JS.html) does. This is useful for doing light DOM manipulation without writing JavaScript, and is a feature of the `phoenix_live_view` JavaScript client protocol.

Currently only `Hide`, `Show`, and `Toggle` are implemented; we plan to implement the remaining utility commands soon.

## License

GoLive is licensed under the [MIT License](./LICENSE).
