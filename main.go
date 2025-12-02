package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/bio-routing/matroschka-prober/pkg/config"
	"github.com/bio-routing/matroschka-prober/pkg/frontend"
	"github.com/bio-routing/matroschka-prober/pkg/probermanager"
	log "github.com/sirupsen/logrus"
	inotify "gopkg.in/fsnotify.v1"

	_ "net/http/pprof"
)

var (
	cfgFilepath = flag.String("config.file", "matroschka.yml", "Config file")
	logLevel    = flag.String("log.level", "error", "Log Level")
)

func main() {
	flag.Parse()

	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalf("Unable to parse log.level: %v", err)
	}
	log.SetLevel(level)

	cfg, err := loadConfig(*cfgFilepath)
	if err != nil {
		log.Fatalf("Unable to load config: %v", err)
	}

	v4Src, err := cfg.GetConfiguredSrcAddr4()
	if err != nil {
		log.Fatalf("unable to get configured IPv4 source address: %v", err)
	}

	v6Src, err := cfg.GetConfiguredSrcAddr6()
	if err != nil {
		log.Fatalf("unable to get configured IPv6 source address: %v", err)
	}

	pm := probermanager.New(*cfg.BasePort, v4Src, v6Src, time.Second)
	err = pm.Configure(cfg)
	if err != nil {
		log.Errorf("reconfiguration failed: %v", err)
	}
	fe := frontend.New(&frontend.Config{
		Version:       cfg.Version,
		MetricsPath:   *cfg.MetricsPath,
		ListenAddress: cfg.ListenAddress.String(),
	}, pm)
	go fe.Start()

	w, err := inotify.NewWatcher()
	if err != nil {
		log.Fatalf("unable to create inotify watcher: %v", err)
	}

	err = w.Add(*cfgFilepath)
	if err != nil {
		log.Fatalf("failed to watch file %q: %v", *cfgFilepath, err)
	}

	for {
		e := <-w.Events

		if e.Op == inotify.Remove {
			continue
		}

		log.Infof("Config has changed: reloading")
		cfg, err := loadConfig(*cfgFilepath)
		if err != nil {
			log.Errorf("unable to reload config: %v", err)
			continue
		}

		err = pm.Configure(cfg)
		if err != nil {
			log.Fatalf("reconfiguration failed: %v", err)
		}
	}
}

func loadConfig(path string) (*config.Config, error) {
	cfgFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %q: %v", path, err)
	}

	cfg := &config.Config{}
	err = yaml.Unmarshal(cfgFile, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal: %v", err)
	}

	err = cfg.ApplyDefaults()
	if err != nil {
		return nil, fmt.Errorf("error applying the defaults: %w", err)
	}

	err = cfg.ConvertIPAddresses()
	if err != nil {
		return nil, fmt.Errorf("error converting IP addresses: %w", err)
	}

	return cfg, nil
}
