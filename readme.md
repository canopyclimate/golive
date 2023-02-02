# GoLive

> **Warning**  
> GoLive is very much **alpha software**. We are just starting to use it in production at [Canopy](https://canopyclimate.com), and are confident that many of the assumptions we made in its initial design will be invalidated in the cold, hard light of reality. The surface API may change under you; however, the core ideas are here to stay.

GoLive is a library for building LiveViews in Go. The LiveView pattern, [as popularized in Elixir’s Phoenix framework](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html), shifts your UI’s state management and event handling to the server, calculating minimal diffs¹ to drive updates in your HTML over websockets.

While we encourage reading the first few paragraphs of the linked Elixir docs as that library is mature and well-documented, here is a short list of advantages to this pattern:

1. Because logic shifts to the server you are no longer reasoning about your web app in terms of a backend, frontend, and connective API tissue: it’s all backend. This means, for example, that it’s perfectly valid to query your database while handling button clicks—no request handling, hooks, or other indirection needed.
2. GoLive (like all flavors of LiveView) includes API for triggering state changes from elsewhere in your server. This allows for live-updating views driven by, say, a Kafka queue, using the exact same API as the rest of your UI and with no additional boilerplate.
3. The server will initially render a static version of your UI, serve it, and then _mount_ that UI by connecting to it via websocket. This means initial page loads are very fast.
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

func (c *Counter) Render(ctx context.Context, meta live.Meta) *htmltmpl.Template {
    return htmltmpl.Must(htmltmpl.New("liveView").Parse(`
        <div>
            <h1>Count is: {{ .Count }}</h1>
            <button phx-click="decrement">-</button>
            <button phx-click="increment">+</button>
        </div>
    `))
}
```

As you can see, the struct itself represents the state of your view. The `phx-click` attributes correspond to event types in our `HandleEvent` handler. After an event is handled, the view is recalculated and new state communicated via websocket to the client where it is displayed.

You can find more examples in the `examples/` directory. To run:

```
go run ./examples/main.go
```

## Installing

```
go get github.com/canopyclimate/golive@latest
```

## About `htmltmpl`

This package includes a fork of the standard library’s `html/template` package named `htmltmpl`. It has similar security guarantees to the original package. The fork exists to support the diffing needed to track changes in your HTML based on the live view’s state.

## Routing

As noted above, GoLive does not endeavor to own your app’s routing. Instead, we provide a few different ways to connect a given route in your app to a live view.

First, you always need to route the path your live views will use to mount themselves via websockets. You can do this by creating a `live.Config` and passing it into `live.NewWebsocketHandler`, giving you an `http.Handler` you can use as you see fit:

```
r.Handle("/live/websocket", live.NewWebsocketHandler(liveConfig))
```

Also in this `live.Config` is a `Mux` property that accepts any `http.Handler` that can mux to your live views. Actually creating the view in your handlers can happen in a few different ways. Each of the options below assume you’re in the context of some handler that has an `http.ResponseWriter` named `w`:

First, you can explicitly cast your response writer to one that understands live views and, if successful, return the view:

```go
if j, ok := w.(*live.JoinHandler); ok {
    j.SetView(new(Counter))
}
```

You may also use this more succinct syntax. If `w` is not a GoLive muxer writer, it is a no-op:

```go
live.SetView(w, new(Counter))
```

This flexibility has some tradeoffs. In addition to letting you bring your own muxer, it also allows opting into patterns in which the URL path contains variables that can then mutate the existing view. Note, however, that those familiar with Phoenix’s LiveView may be surprised at how path variables are parsed at the _routing_ layer rather than internally as part of `HandleParams`. Here’s an example using [gorilla/mux](https://github.com/gorilla/mux):

Let’s imagine a route that could start our `Counter` at a number in the route, like `/counter/12`:

```go
livemux := mux.NewRouter()
livemux.HandleFunc("/counter/{i}", func(w http.ResponseWriter, r *http.Request) {
    x := try.E1(strconv.Atoi(mux.Vars(r)["i"]))
    c := live.MakeView[*Counter](w) // Note this line.
    c.Count = x
    live.SetView(w, c)
})
```

Note the annotated line: the `live.MakeView` helper will return the existing live view for this session if it already exists, and otherwise create it. We then set the count value.

> **Note**  
> When you patch a view in GoLive, we first give you an opportunity to re-handle the “request,” parsing it as needed, before calling `HandleParams`. In Phoenix terms, path params are handled different from URL query params: path params are parsed out at the muxer layer, URL query params in the more traditional `HandleParams` callback. This is a consequence of our decision to let you bring your own muxer, but may be unexpected for those familiar with Phoenix.

Once you’ve set up your muxer as desired, remember to set it as the `Mux` property on `live.Config`.

## License

GoLive is licensed under the [MIT License](./LICENSE).
