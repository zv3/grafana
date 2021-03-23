// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/grafana/grafana/pkg/api"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/server"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/rendering"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb"
)

var wireSet = wire.NewSet(
	tsdb.NewService,
	wire.Bind(new(plugins.DataRequestHandler), new(*tsdb.Service)),
	alerting.ProvideAlertEngine,
	setting.NewCfg,
	server.New,
	api.ProvideHTTPServer,
	bus.ProvideBus,
	wire.Bind(new(bus.Bus), new(*bus.InProcBus)),
	rendering.ProvideService,
	wire.Bind(new(rendering.Service), new(*rendering.RenderingService)),
	routing.ProvideRegister,
	wire.Bind(new(routing.RouteRegister), new(*routing.RouteRegisterImpl)),
)

func initializeServer(cla setting.CommandLineArgs, opts server.Options, apiOpts api.ServerOptions) (*server.Server, error) {
	wire.Build(wireSet)
	return &server.Server{}, nil
}
