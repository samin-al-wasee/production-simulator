package sandbox

// Templates offered when an application instance is placed (v6). The engine
// never reads them: a template only fills in an AppConfig, which is validated
// like any other. An application type is route names, costs, dependencies,
// and typical shares; the economy stays generic (Principle 7).

// AppStack is how an application serves: a complete configuration whose
// routes the chosen application type replaces.
type AppStack struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	App         AppConfig `json:"app"`
}

// AppType is what an application serves: its routes.
type AppType struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Routes      []AppRoute `json:"routes"`
}

func appStacks(base *AppConfig) []AppStack {
	stack := func(name, desc, framework, iface, server, processing string, port, workers, concurrency int, mw ...string) AppStack {
		a := *base
		a.Routes = append([]AppRoute(nil), base.Routes...)
		a.Framework, a.Interface, a.Server, a.Processing = framework, iface, server, processing
		a.Port, a.Workers, a.MaxConcurrency, a.Middleware = port, workers, concurrency, mw
		return AppStack{Name: name, Description: desc, App: a}
	}
	return []AppStack{
		stack("Django", "Sync Python: 4 Gunicorn workers, each held by a request while it waits on the database.",
			"Python / Django", "WSGI", "Gunicorn", ProcessingSync, 8000, 4, 4, "request-id", "logging", "auth", "validation"),
		stack("FastAPI", "Async Python: 2 Uvicorn workers keep up to 500 requests in flight while they wait.",
			"Python / FastAPI", "ASGI", "Uvicorn", ProcessingAsync, 8000, 2, 500, "request-id", "logging", "auth", "validation"),
		stack("Express", "One Node.js event loop: a single core, many requests in flight, compressed responses.",
			"Node.js / Express", "HTTP", "Node", ProcessingAsync, 3000, 1, 1000, "request-id", "logging", "cors", "compression"),
		stack("Rails", "Sync Ruby: 4 Puma workers with the full middleware stack.",
			"Ruby / Rails", "Rack", "Puma", ProcessingSync, 3000, 4, 4, "request-id", "logging", "cors", "auth", "validation"),
		stack("Go", "Async Go: up to 5,000 requests in flight on 2 workers, with light middleware.",
			"Go / net/http", "HTTP", "net/http", ProcessingAsync, 8080, 2, 5000, "request-id", "logging"),
	}
}

// appRoute is shorthand for a template route: base and CPU time (ms), memory
// (MB), request and response size (KB), typical share, and dependencies.
func appRoute(endpoint string, baseMs, cpuMs, memMB, reqKB, respKB, share float64, deps ...string) AppRoute {
	return AppRoute{Endpoint: endpoint, BaseMs: baseMs, CPUMs: cpuMs, MemoryMB: memMB,
		RequestKB: reqKB, ResponseKB: respKB, Share: share, Deps: deps}
}

func appTypes() []AppType {
	login := appRoute("POST /login", 30, 25, 1, 1, 1, 0, DepCache)
	all := appRoute(CatchAll, 20, 15, 2, 1, 5, 0, DepCache)
	with := func(r AppRoute, share float64) AppRoute { r.Share = share; return r }
	return []AppType{
		{Name: "E-commerce", Description: "Browsing a catalog with images, a profile, and orders: read-heavy and cache-friendly.",
			Routes: []AppRoute{
				appRoute("GET /products", 20, 15, 2, 1, 20, 0.35, DepCache),
				appRoute("GET /products/:id", 15, 12, 1, 1, 5, 0.2, DepCache),
				appRoute("GET /profile", 15, 10, 1, 1, 2, 0.1, DepCache),
				appRoute("GET /media/:id", 25, 10, 4, 1, 200, 0.15, DepCache, DepStorage),
				appRoute("POST /orders", 40, 30, 3, 4, 2, 0.12, DepDBWrite),
				with(login, 0.08), all,
			}},
		{Name: "Flight booking", Description: "Expensive searches dominate; bookings and payments are few but slow and must reach the primary.",
			Routes: []AppRoute{
				appRoute("GET /flights/search", 120, 40, 8, 2, 40, 0.45, DepCache, DepDBRead),
				appRoute("GET /flights/:id", 20, 10, 2, 1, 10, 0.2, DepCache),
				appRoute("POST /bookings", 80, 30, 4, 4, 4, 0.08, DepDBWrite),
				appRoute("GET /bookings/:id", 20, 10, 1, 1, 5, 0.1, DepDBRead),
				appRoute("POST /payments", 150, 20, 2, 2, 2, 0.05, DepDBWrite),
				with(login, 0.12), all,
			}},
		{Name: "Ride sharing", Description: "Drivers report their location every few seconds: a flood of tiny writes next to nearby-driver lookups.",
			Routes: []AppRoute{
				appRoute("POST /drivers/location", 5, 3, 0.5, 0.5, 0.2, 0.5, DepDBWrite),
				appRoute("GET /drivers/nearby", 40, 30, 4, 1, 8, 0.2, DepCache, DepDBRead),
				appRoute("POST /rides", 60, 25, 3, 2, 3, 0.05, DepDBWrite),
				appRoute("GET /rides/:id", 15, 8, 1, 1, 3, 0.15, DepCache),
				appRoute("POST /payments", 150, 20, 2, 2, 2, 0.03, DepDBWrite),
				with(login, 0.07), all,
			}},
		{Name: "Social feed", Description: "Feeds are heavy reads, likes are many small writes, and media fills the network.",
			Routes: []AppRoute{
				appRoute("GET /feed", 50, 30, 6, 1, 60, 0.4, DepCache, DepDBRead),
				appRoute("GET /posts/:id", 15, 10, 2, 1, 10, 0.15, DepCache),
				appRoute("POST /posts", 40, 25, 3, 8, 2, 0.05, DepDBWrite, DepStorage),
				appRoute("POST /likes", 8, 5, 0.5, 0.5, 0.2, 0.25, DepDBWrite),
				appRoute("GET /media/:id", 25, 10, 4, 1, 300, 0.1, DepCache, DepStorage),
				with(login, 0.05), all,
			}},
		{Name: "Video streaming", Description: "Video segments are cheap on CPU but huge: the network runs out long before the processor.",
			Routes: []AppRoute{
				appRoute("GET /catalog", 30, 20, 4, 1, 30, 0.15, DepCache),
				appRoute("GET /videos/:id", 20, 12, 2, 1, 8, 0.1, DepCache),
				appRoute("GET /videos/:id/segment", 15, 5, 8, 0.5, 1000, 0.6, DepStorage),
				appRoute("POST /views", 8, 5, 0.5, 1, 0.2, 0.1, DepDBWrite),
				with(login, 0.05), all,
			}},
	}
}

// microservices are application types that split the e-commerce API into
// services calling each other by name (v7): a storefront in front of a
// catalog, orders, and payments. The chooser names each "<type> API", which
// is the name the calls use.
func microservices() []AppType {
	call := func(r AppRoute, calls ...Call) AppRoute { r.Calls = calls; return r }
	return []AppType{
		{Name: "Storefront", Description: "A backend for the frontend: no data of its own, it calls the Catalog, Orders, and Payments services.",
			Routes: []AppRoute{
				call(appRoute("GET /products", 10, 5, 1, 1, 20, 0.45), Call{Service: "Catalog API", Endpoint: "GET /products"}),
				call(appRoute("GET /products/:id", 10, 5, 1, 1, 5, 0.3), Call{Service: "Catalog API", Endpoint: "GET /products/:id"}),
				call(appRoute("POST /checkout", 20, 10, 2, 4, 2, 0.15),
					Call{Service: "Orders API", Endpoint: "POST /orders"}, Call{Service: "Payments API", Endpoint: "POST /charge"}),
				appRoute("POST /login", 30, 25, 1, 1, 1, 0.1, DepCache),
			}},
		{Name: "Catalog", Description: "Products and their details, read through a cache.",
			Routes: []AppRoute{
				appRoute("GET /products", 20, 15, 2, 1, 20, 0.6, DepCache),
				appRoute("GET /products/:id", 15, 12, 1, 1, 5, 0.4, DepCache),
			}},
		{Name: "Orders", Description: "Order writes to its own database, then an async call to notify the customer.",
			Routes: []AppRoute{
				call(appRoute("POST /orders", 40, 30, 3, 4, 2, 0.7, DepDBWrite), Call{Service: "Notifications API", Endpoint: "POST /notify", Async: true}),
				appRoute("GET /orders/:id", 15, 10, 1, 1, 3, 0.3, DepDBRead),
			}},
		{Name: "Payments", Description: "Slow charges against an outside processor, recorded in its own database.",
			Routes: []AppRoute{appRoute("POST /charge", 300, 20, 2, 2, 1, 1, DepDBWrite)}},
		{Name: "Notifications", Description: "Sends emails and pushes; nothing waits for it.",
			Routes: []AppRoute{appRoute("POST /notify", 50, 10, 1, 2, 1, 1)}},
	}
}
