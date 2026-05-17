package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/fisherevans/meatbag/internal/blobs"
	"github.com/fisherevans/meatbag/internal/render"
	"github.com/fisherevans/meatbag/internal/secrets"
	"github.com/fisherevans/meatbag/internal/store"
	"github.com/fisherevans/meatbag/internal/tree"
	"github.com/fisherevans/meatbag/internal/ui"
)

// Server holds the daemon's runtime dependencies.
type Server struct {
	Store     *store.Store
	Blobs     *blobs.Store
	Broker    *broker
	UIFS      fs.FS
	StartedAt time.Time
	Version   string

	// stopping is closed when Run begins shutdown so SSE handlers can exit
	// promptly instead of blocking http.Server.Shutdown forever.
	stopping chan struct{}
}

// New wires a server. fsnotify watching is started by Run.
func New(s *store.Store) (*Server, error) {
	bs, err := blobs.New(s.BlobsDir())
	if err != nil {
		return nil, err
	}
	return &Server{
		Store:     s,
		Blobs:     bs,
		Broker:    newBroker(),
		UIFS:      ui.FS(),
		StartedAt: time.Now().UTC(),
		Version:   "0.1.0",
		stopping:  make(chan struct{}),
	}, nil
}

// Run starts the HTTP listener on addr and the fsnotify watcher. Blocks until
// ctx is canceled or the listener errors.
func (srv *Server) Run(ctx context.Context, addr string) error {
	mux := srv.routes()
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := watchLists(ctx, srv.Store, srv.Broker); err != nil {
			fmt.Println("watch error:", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		err := httpSrv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		close(srv.stopping) // wake SSE handlers so Shutdown isn't blocked by them
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			// Force-close anything still hanging.
			return httpSrv.Close()
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func (srv *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "uptime_s": int(time.Since(srv.StartedAt).Seconds())})
	})
	mux.HandleFunc("GET /api/lists", srv.handleLists)
	mux.HandleFunc("GET /api/lists/{slug}", srv.handleList)
	mux.HandleFunc("POST /api/lists/{slug}/items/{itemID}/state", srv.handleItemState)
	mux.HandleFunc("POST /api/lists/{slug}/items/{itemID}/inputs/{field}", srv.handleSetInput)
	mux.HandleFunc("DELETE /api/lists/{slug}/items/{itemID}/inputs/{field}", srv.handleClearInput)
	mux.HandleFunc("GET /api/blobs/{sha}", srv.handleBlobGet)
	mux.HandleFunc("GET /api/events", srv.handleSSE)

	if srv.UIFS != nil {
		fileServer := http.FileServer(http.FS(srv.UIFS))
		mux.Handle("/", srv.spaHandler(fileServer))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!doctype html><meta charset=utf-8><title>meatbag</title>
<style>body{font:14px system-ui;margin:2em;max-width:40em}</style>
<h1>meatbag</h1>
<p>The UI was not embedded into this binary. Run <code>make ui</code> and rebuild,
or hit the <a href="/api/lists">JSON API</a> directly.</p>`)
		})
	}
	return loggingMiddleware(mux)
}

func (srv *Server) spaHandler(fileServer http.Handler) http.Handler {
	root, _ := fs.Sub(srv.UIFS, ".")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API/static path: try the file system first; fall back to index.html
		// for SPA routes.
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if root != nil {
			if _, err := fs.Stat(root, clean); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback: serve index.html
		f, err := srv.UIFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fileCopy(w, f)
	})
}

func fileCopy(w http.ResponseWriter, f fs.File) (int64, error) {
	buf := make([]byte, 4096)
	var total int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			if errors.Is(err, fs.ErrClosed) || err.Error() == "EOF" {
				return total, nil
			}
			return total, err
		}
	}
}

func loggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		dur := time.Since(start)
		fmt.Printf("%s %s %s\n", r.Method, r.URL.Path, dur)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ----- list handlers -----

type listRow struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	ProjectPath string `json:"project_path,omitempty"`
	Status      string `json:"status"`
	UpdatedAt   string `json:"updated_at"`
	Progress    progress `json:"progress"`
}

type progress struct {
	Todo          int `json:"todo"`
	InProgress    int `json:"in_progress"`
	Blocked       int `json:"blocked"`
	Done          int `json:"done"`
	Skipped       int `json:"skipped"`
	AwaitingInput int `json:"awaiting_input"`
}

func progressOf(list *store.List) progress {
	var p progress
	var walk func(items []*store.Item)
	walk = func(items []*store.Item) {
		for _, it := range items {
			switch it.State {
			case store.StateTodo:
				p.Todo++
			case store.StateInProgress:
				p.InProgress++
			case store.StateBlocked:
				p.Blocked++
			case store.StateDone:
				p.Done++
			case store.StateSkipped:
				p.Skipped++
			}
			for _, in := range it.Inputs {
				v, ok := it.InputValues[in.Name]
				if in.Required && (!ok || !v.HasValue) && it.State != store.StateDone && it.State != store.StateSkipped {
					p.AwaitingInput++
					break
				}
			}
			walk(it.Children)
		}
	}
	walk(list.Items)
	return p
}

func (srv *Server) handleLists(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}
	lists, err := srv.Store.ListAll(status)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	rows := make([]listRow, 0, len(lists))
	for _, l := range lists {
		rows = append(rows, listRow{
			ID: l.ID, Slug: l.Slug, Title: l.Title,
			ProjectPath: l.ProjectPath, Status: string(l.Status),
			UpdatedAt: l.UpdatedAt.Format(time.RFC3339),
			Progress:  progressOf(l),
		})
	}
	writeJSON(w, 200, rows)
}

// listItemDTO mirrors store.Item but adds the derived label and renders
// markdown content/description to safe HTML server-side.
type listItemDTO struct {
	ID          string                      `json:"id"`
	Label       string                      `json:"label"`
	Title       string                      `json:"title"`
	Owner       string                      `json:"owner"`
	State       string                      `json:"state"`
	Content     string                      `json:"content,omitempty"`
	ContentHTML string                      `json:"content_html,omitempty"`
	Inputs      []store.Input               `json:"inputs,omitempty"`
	InputValues map[string]store.InputValue `json:"input_values,omitempty"`
	Note        string                      `json:"note,omitempty"`
	Children    []listItemDTO               `json:"children,omitempty"`
}

type listDTO struct {
	ID              string        `json:"id"`
	Slug            string        `json:"slug"`
	Title           string        `json:"title"`
	Description     string        `json:"description,omitempty"`
	DescriptionHTML string        `json:"description_html,omitempty"`
	ProjectPath     string        `json:"project_path,omitempty"`
	Status          string        `json:"status"`
	UpdatedAt       string        `json:"updated_at"`
	Items           []listItemDTO `json:"items"`
}

func toItemDTO(it *store.Item, labels map[string]string) listItemDTO {
	dto := listItemDTO{
		ID: it.ID, Label: labels[it.ID], Title: it.Title,
		Owner: string(it.Owner), State: string(it.State),
		Content: it.Content, ContentHTML: render.Markdown(it.Content),
		Inputs: it.Inputs, InputValues: redactInputs(it.InputValues),
		Note: it.Note,
	}
	for _, c := range it.Children {
		dto.Children = append(dto.Children, toItemDTO(c, labels))
	}
	return dto
}

// redactInputs strips secret values from input_values so the UI never receives
// raw secrets. HasValue is preserved so the UI can render a "set" indicator.
func redactInputs(in map[string]store.InputValue) map[string]store.InputValue {
	if in == nil {
		return nil
	}
	out := make(map[string]store.InputValue, len(in))
	for k, v := range in {
		if v.SecretRef != "" {
			out[k] = store.InputValue{SecretRef: v.SecretRef, HasValue: v.HasValue}
		} else {
			out[k] = v
		}
	}
	return out
}

func (srv *Server) loadBySlug(slug string) (*store.List, error) {
	path, _, err := srv.Store.FindPath(slug)
	if err != nil {
		return nil, err
	}
	return srv.Store.LoadList(path)
}

func (srv *Server) handleList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	l, err := srv.loadBySlug(slug)
	if err != nil {
		writeErr(w, 404, "list not found")
		return
	}
	labels := tree.Labels(l)
	dto := listDTO{
		ID: l.ID, Slug: l.Slug, Title: l.Title,
		Description: l.Description, DescriptionHTML: render.Markdown(l.Description),
		ProjectPath: l.ProjectPath, Status: string(l.Status),
		UpdatedAt: l.UpdatedAt.Format(time.RFC3339),
	}
	for _, it := range l.Items {
		dto.Items = append(dto.Items, toItemDTO(it, labels))
	}
	writeJSON(w, 200, dto)
}

func (srv *Server) handleItemState(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	itemID := r.PathValue("itemID")
	var body struct {
		State string `json:"state"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	state := store.State(body.State)
	if !store.ValidState(state) {
		writeErr(w, 400, "invalid state")
		return
	}
	err := srv.Store.Update(slug, func(l *store.List) error {
		it, _, _, ok := tree.FindByID(l, itemID)
		if !ok {
			return store.ErrNotFound
		}
		it.State = state
		if body.Note != "" {
			it.Note = body.Note
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, 404, "item not found")
			return
		}
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func (srv *Server) handleSetInput(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	itemID := r.PathValue("itemID")
	field := r.PathValue("field")

	// Resolve list + schema first (to know the field type).
	l, err := srv.loadBySlug(slug)
	if err != nil {
		writeErr(w, 404, "list not found")
		return
	}
	it, _, _, ok := tree.FindByID(l, itemID)
	if !ok {
		writeErr(w, 404, "item not found")
		return
	}
	var schema *store.Input
	for i := range it.Inputs {
		if it.Inputs[i].Name == field {
			schema = &it.Inputs[i]
			break
		}
	}
	if schema == nil {
		writeErr(w, 400, "no such input field")
		return
	}

	var newVal store.InputValue
	switch schema.Type {
	case "password":
		var body struct{ Value string `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "invalid body")
			return
		}
		ref, err := secrets.Set(l.ID, it.ID, field, body.Value)
		if err != nil {
			writeErr(w, 500, "keychain: "+err.Error())
			return
		}
		newVal = store.InputValue{SecretRef: ref, HasValue: true}
	case "file":
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeErr(w, 400, "expected multipart form: "+err.Error())
			return
		}
		f, header, err := r.FormFile("file")
		if err != nil {
			writeErr(w, 400, "missing file field")
			return
		}
		defer f.Close()
		sha, size, err := srv.Blobs.Write(f)
		if err != nil {
			writeErr(w, 500, "blob: "+err.Error())
			return
		}
		newVal = store.InputValue{
			BlobRef:  blobs.BuildRef(sha),
			Filename: filepath.Base(header.Filename),
			Size:     size,
			HasValue: true,
		}
	case "number":
		var body struct{ Value float64 `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "invalid body")
			return
		}
		newVal = store.InputValue{Value: body.Value, HasValue: true}
	case "checkbox":
		var body struct{ Value bool `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "invalid body")
			return
		}
		newVal = store.InputValue{Value: body.Value, HasValue: true}
	case "multiselect":
		var body struct{ Value []string `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "invalid body")
			return
		}
		newVal = store.InputValue{Value: body.Value, HasValue: true}
	default:
		var body struct{ Value string `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "invalid body")
			return
		}
		newVal = store.InputValue{Value: body.Value, HasValue: true}
	}

	err = srv.Store.Update(slug, func(l *store.List) error {
		it, _, _, ok := tree.FindByID(l, itemID)
		if !ok {
			return store.ErrNotFound
		}
		if it.InputValues == nil {
			it.InputValues = map[string]store.InputValue{}
		}
		if prev, ok := it.InputValues[field]; ok {
			cleanupValue(prev, srv.Blobs)
		}
		it.InputValues[field] = newVal
		return nil
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"has_value": true})
}

func (srv *Server) handleClearInput(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	itemID := r.PathValue("itemID")
	field := r.PathValue("field")
	err := srv.Store.Update(slug, func(l *store.List) error {
		it, _, _, ok := tree.FindByID(l, itemID)
		if !ok {
			return store.ErrNotFound
		}
		if prev, ok := it.InputValues[field]; ok {
			cleanupValue(prev, srv.Blobs)
			delete(it.InputValues, field)
		}
		return nil
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "true"})
}

func cleanupValue(v store.InputValue, bs *blobs.Store) {
	if v.SecretRef != "" {
		if ref, err := secrets.ParseRef(v.SecretRef); err == nil {
			_ = secrets.Delete(ref)
		}
	}
	if v.BlobRef != "" && bs != nil {
		if sha, err := blobs.ParseRef(v.BlobRef); err == nil {
			_ = bs.Delete(sha)
		}
	}
}

func (srv *Server) handleBlobGet(w http.ResponseWriter, r *http.Request) {
	sha := r.PathValue("sha")
	if len(sha) != 64 {
		writeErr(w, 400, "invalid sha")
		return
	}
	f, err := srv.Blobs.Read(sha)
	if err != nil {
		writeErr(w, 404, "not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	if name := r.URL.Query().Get("filename"); name != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}
	_, _ = fileCopy(w, f.(fs.File))
}

func (srv *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := srv.Broker.subscribe()
	defer srv.Broker.unsubscribe(ch)

	// Initial ping so clients know they're connected.
	fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
	flusher.Flush()

	pingT := time.NewTicker(20 * time.Second)
	defer pingT.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-srv.stopping:
			return
		case <-pingT.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.marshal())
			flusher.Flush()
		}
	}
}
