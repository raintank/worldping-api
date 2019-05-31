package main

import (
	"flag"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Dieterbe/profiletrigger/heap"
	"github.com/raintank/met/helper"
	"github.com/raintank/raintank-probe/publisher"
	"github.com/raintank/worldping-api/pkg/alerting"
	"github.com/raintank/worldping-api/pkg/api"
	"github.com/raintank/worldping-api/pkg/cmd"
	"github.com/raintank/worldping-api/pkg/events"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	"github.com/raintank/worldping-api/pkg/services/endpointdiscovery"
	"github.com/raintank/worldping-api/pkg/services/notifications"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
	"github.com/raintank/worldping-api/pkg/setting"
)

var version = "master"
var commit = "NA"
var buildstamp string

var configFile = flag.String("config", "", "path to config file")
var homePath = flag.String("homepath", "", "path to grafana install/home path, defaults to working directory")
var exitChan = make(chan int)

func main() {
	buildstampInt64, _ := strconv.ParseInt(buildstamp, 10, 64)

	setting.BuildVersion = version
	setting.BuildCommit = commit
	setting.BuildStamp = buildstampInt64
	notifyShutdown := make(chan struct{})
	go listenToSystemSignels(notifyShutdown)

	flag.Parse()
	initRuntime()

	if setting.ProfileHeapMB > 0 {
		errors := make(chan error)
		go func() {
			for e := range errors {
				log.Error(0, e.Error())
			}
		}()
		heap, _ := heap.New(setting.ProfileHeapDir, setting.ProfileHeapMB*1000000, setting.ProfileHeapWait, time.Duration(1)*time.Second, errors)
		go heap.Run()
	}

	metricsBackend, err := helper.New(setting.StatsdEnabled, setting.StatsdAddr, setting.StatsdType, "worldping-api", setting.InstanceId)
	if err != nil {
		log.Error(3, "Statsd client:", err)
	}

	events.Init()
	tsdbUrl, _ := url.Parse(setting.TsdbUrl)
	tsdbPublisher := publisher.NewTsdb(tsdbUrl, setting.AdminKey, 1)
	api.InitCollectorController(metricsBackend, tsdbPublisher)
	if setting.Alerting.Enabled {
		alerting.Init(metricsBackend, tsdbPublisher)
		alerting.Construct()
	}

	if err := notifications.Init(); err != nil {
		log.Fatal(3, "Notification service failed to initialize", err)
	}

	if err := endpointdiscovery.InitEndpointDiscovery(); err != nil {
		log.Fatal(3, "EndpointDiscovery service failed to initialize.", err)
	}

	cmd.StartServer(notifyShutdown)
	exitChan <- 0
}

func initRuntime() {
	err := setting.NewConfigContext(&setting.CommandLineArgs{
		Config:   *configFile,
		HomePath: *homePath,
		Args:     flag.Args(),
	})

	if err != nil {
		log.Fatal(3, err.Error())
	}

	log.Info("Starting worldping-api")
	log.Info("Version: %v, Commit: %v, Build date: %v", setting.BuildVersion, setting.BuildCommit, time.Unix(setting.BuildStamp, 0))
	setting.LogConfigurationInfo()

	sqlstore.NewEngine()
	middleware.Init(setting.AdminKey)
}

func listenToSystemSignels(notifyShutdown chan struct{}) {
	signalChan := make(chan os.Signal, 1)
	code := 0

	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)
	signal.Notify(signalChan, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		log.Info("Received signal %s. shutting down", sig)
	case code = <-exitChan:
		switch code {
		case 0:
			log.Info("Shutting down")
		default:
			log.Warn("Shutting down")
		}
	}
	close(notifyShutdown)

	publisher.Publisher.Close()
	api.ShutdownController()
	log.Close()
	os.Exit(code)
}
