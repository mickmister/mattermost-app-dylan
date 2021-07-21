package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

type Handler func(c *apps.CallRequest, callType string, p *Plugin) *apps.CallResponse

type BindingEntry struct {
	entries []*BindingEntry
	binding *apps.Binding
	handler Handler
}

type BindingRegister struct {
	entries []*BindingEntry
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if !strings.HasPrefix(path, "/app") {
		http.NotFound(w, r)
	}

	path = path[len("/app"):]

	switch path {
	case "/bindings":
		p.handleBindings(w, r)
		return
	case "/manifest":
		p.handleManifest(w, r)
		return
	}

	if strings.HasPrefix(path, "/static") {
		p.handleStatic(w, r)
		return
	}

	handler := register.GetHandler(path)
	if handler != nil {
		p.handleCallRequest(w, r, handler)
		return
	}

	http.NotFound(w, r)
}

func (p *Plugin) handleCallRequest(w http.ResponseWriter, r *http.Request, handler Handler) {
	c := &apps.CallRequest{}

	err := json.NewDecoder(r.Body).Decode(c)
	if err != nil {
		err = errors.Wrap(err, "error unmarshaling call request")
		p.writeErrorCallResponse(w, err)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	callType := parts[len(parts)-1]
	res := handler(c, callType, p)
	httputils.WriteJSON(w, res)
}

func (p *Plugin) handleBindings(w http.ResponseWriter, r *http.Request) {
	bindings := register.GetBindings()

	res := apps.CallResponse{
		Type: apps.CallResponseTypeOK,
		Data: bindings,
	}

	httputils.WriteJSON(w, res)
}

func (p *Plugin) writeErrorCallResponse(w http.ResponseWriter, err error) {
	p.API.LogError(err.Error())
	httputils.WriteJSON(w, apps.NewErrorCallResponse(err))
}

func (r *BindingRegister) GetBindings() []apps.Binding {
	bindings := []apps.Binding{}

	for _, entry := range r.entries {
		bindings = append(bindings, entry.GetBinding())
	}

	return bindings
}

func (e *BindingEntry) GetBinding() apps.Binding {
	b := *e.binding

	for _, e2 := range e.entries {
		b2 := e2.GetBinding()
		b.Bindings = append(b.Bindings, &b2)
	}

	return b
}

func (r *BindingRegister) GetHandler(path string) Handler {
	path = strings.TrimSuffix(path, "/submit")
	path = strings.TrimSuffix(path, "/form")
	path = strings.TrimSuffix(path, "/lookup")

	for _, e := range r.entries {
		handler := e.GetHandler(path)
		if handler != nil {
			return handler
		}
	}

	return nil
}

func (e *BindingEntry) GetHandler(path string) Handler {
	if e.binding.Call != nil && e.binding.Call.Path == path {
		return e.handler
	}

	if e.binding.Form != nil && e.binding.Form.Call != nil && e.binding.Form.Call.Path == path {
		return e.handler
	}

	for _, e2 := range e.entries {
		h := e2.GetHandler(path)
		if h != nil {
			return h
		}
	}

	return nil
}

func (r *BindingRegister) AddEntry(e *BindingEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

func (e *BindingEntry) AddEntry(e2 *BindingEntry) error {
	e.entries = append(e.entries, e2)
	return nil
}
