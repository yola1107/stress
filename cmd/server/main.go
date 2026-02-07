package main

import (
	"flag"
	"os"
	"time"

	"stress/internal/conf"
	"stress/pkg/zap"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name     = "stress"
	Version  = "v0.0.1"
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")

	time.Local = initLocation()
}

// initLocation 初始化时区，失败时使用UTC
func initLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Warnf("无法加载 Asia/Shanghai 时区，使用 UTC: %v", err)
		return time.UTC
	}
	return loc
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()

	log.Infof("Starting server. Name=%q, Version=%q", Name, Version)

	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	logger := zap.NewLoggerWithConfig(&zap.Config{
		Mode:  zap.Mode(bc.Log.Mode), // Use production mode for file logging
		Level: bc.Log.Level,          // Set log level
		App:   bc.Log.App,            // Application name
		Dir:   bc.Log.Dir,            // Log directory
		File:  bc.Log.File,           // Log file Open
	})
	defer logger.Sync()

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Stress, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
