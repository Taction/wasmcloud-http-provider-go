package server

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wasmCloud/provider-sdk-go"
	core "github.com/wasmcloud/interfaces/core/tinygo"
	httpserver "github.com/wasmcloud/interfaces/httpserver/tinygo"
	msgpack "github.com/wasmcloud/tinygo-msgpack"
)

type HttpServerInterface interface {
	Run() error
	Shutdown() error
}

type HttpServer struct {
	server *http.Server
	ld     core.LinkDefinition
	logger logrus.FieldLogger
	p      *provider.WasmcloudProvider
}

// Make sure *HttpServer satisfies the HttpServerInterface
var _ HttpServerInterface = (*HttpServer)(nil)

func New(p *provider.WasmcloudProvider, conf core.LinkDefinition, logger logrus.FieldLogger) *HttpServer {
	return &HttpServer{p: p, ld: conf, logger: logger}
}

func (h *HttpServer) Run() error {
	address := h.ld.Values["address"]
	h.server = &http.Server{Addr: address, Handler: h}
	go func() {
		err := h.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			h.logger.Errorf("Error starting server for actor [%s] err: %s", h.ld.ActorId, err)
		}
	}()
	return nil
}

func (h *HttpServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	err := h.server.Shutdown(ctx)
	h.logger.Errorf("Error shutting down server for actor [%s] err: %s", h.ld.ActorId, err)
	cancel()
	return err
}

func (h *HttpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Infof("Received request")
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	req, err := transferRequest(r)
	if err != nil {
		h.handleError(w, err)
		return
	}
	h.logger.Infof("Sending request to actor with request: %+v", req)
	body := provider.MEncode(req)
	res, err := h.p.ToActor(h.ld.ActorId, body, "HttpServer.HandleRequest")
	if err != nil {
		h.handleError(w, err)
		return
	}
	resp := httpserver.HttpResponse{}
	b := msgpack.NewDecoder(res)
	resp, err = httpserver.MDecodeHttpResponse(&b)
	if err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(int(resp.StatusCode))
	if len(resp.Header) > 0 {
		addHeaders(w, resp.Header)
	}
	w.Write(resp.Body)
}

func (h *HttpServer) handleError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func transferRequest(r *http.Request) (*httpserver.HttpRequest, error) {
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return nil, err
	}
	h := httpserver.HeaderMap{}
	for k, v := range r.Header {
		h[k] = httpserver.HeaderValues(v)
	}
	return &httpserver.HttpRequest{
		Method: r.Method,
		Path:   r.URL.String(),
		Body:   body,
		Header: h,
	}, nil
}

func addHeaders(w http.ResponseWriter, headers httpserver.HeaderMap) {
	for k, v := range headers {
		for _, tV := range v {
			if w.Header().Get(k) == "" {
				w.Header().Set(k, tV)
			} else {
				w.Header().Add(k, tV)
			}
		}
	}
}
