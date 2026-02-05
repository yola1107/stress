//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"stress/internal/biz"
	"stress/internal/biz/chart"
	"stress/internal/conf"
	"stress/internal/data"
	"stress/internal/notify"
	"stress/internal/server"
	"stress/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *conf.Stress, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, notify.ProviderSet, chart.ProviderSet, service.ProviderSet, newApp))
}
