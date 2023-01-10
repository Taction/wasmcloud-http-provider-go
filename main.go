package main

import (
	"sync"

	"github.com/sirupsen/logrus"
	provider "github.com/wasmCloud/provider-sdk-go"
	core "github.com/wasmcloud/interfaces/core/tinygo"

	"github.com/jordan-rash/tnet-httpserver/server"
)

type HttpServerProvider struct {
	l        sync.Mutex
	Actors   map[string]server.HttpServerInterface
	Provider *provider.WasmcloudProvider
	Logger   logrus.FieldLogger
}

func NewHttpServerProvider() *HttpServerProvider {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	return &HttpServerProvider{
		Actors: make(map[string]server.HttpServerInterface),
		Logger: log,
	}
}

func main() {
	var err error
	p := NewHttpServerProvider()

	prov, err := provider.New(
		"wasmcloud:httpserver",
		provider.WithNewLinkFunc(p.PutLink),
		provider.WithDelLinkFunc(p.DeleteLink),
		provider.WithShutdownFunc(p.Shutdown),
		provider.WithHealthCheckMsg(p.healthCheckMsg),
	)
	if err != nil {
		panic(err)
	}
	p.Provider = prov

	err = prov.Start()
	if err != nil {
		panic(err)
	}
}

func (p *HttpServerProvider) healthCheckMsg() string {
	return ""
}

func (p *HttpServerProvider) PutLink(l core.LinkDefinition) error {
	s := server.New(p.Provider, l, p.Logger)
	err := s.Run()
	if err != nil {
		return err
	}
	p.l.Lock()
	p.Actors[l.ActorId] = s
	p.l.Unlock()
	return nil
}

func (p *HttpServerProvider) DeleteLink(l core.LinkDefinition) error {
	actorID := l.ActorId
	p.l.Lock()
	s := p.Actors[actorID]
	delete(p.Actors, actorID)
	p.l.Unlock()
	go func() {
		if s == nil {
			return
		}
		s.Shutdown()
	}()
	return nil
}

func (p *HttpServerProvider) Shutdown() error {
	p.l.Lock()
	for _, s := range p.Actors {
		s.Shutdown()
	}
	p.Actors = make(map[string]server.HttpServerInterface)
	p.l.Unlock()
	return nil
}
