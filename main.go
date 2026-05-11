package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

	reloadFailed := config.NewReloadFailed()

	pm := probermanager.New(*cfg.BasePort, v4Src, v6Src, time.Second, cfg.Rmem)
	err = pm.Configure(cfg)
	if err != nil {
		log.Errorf("reconfiguration failed: %v", err)
		reloadFailed.SetFailed()
	}

	fe := frontend.New(&frontend.Config{
		Version:       cfg.Version,
		MetricsPath:   *cfg.MetricsPath,
		ListenAddress: cfg.ListenAddress.String(),
	}, pm, reloadFailed)
	go fe.Start()

	w, err := inotify.NewWatcher()
	if err != nil {
		log.Fatalf("unable to create inotify watcher: %v", err)
	}

	// Watch the parent directory rather than the config file itself: inotify
	// watches are bound to inodes, so an atomic rename
	// would orphan a watch on the file and silently break
	// reloads. Watching the directory lets us see Create/Write/Rename events
	// for the replacement file.
	cfgDir, cfgName := filepath.Split(*cfgFilepath)
	if cfgDir == "" {
		cfgDir = "."
	}
	err = w.Add(cfgDir)
	if err != nil {
		log.Fatalf("failed to watch directory %q: %v", cfgDir, err)
	}

	for e := range w.Events {
		if filepath.Base(e.Name) != cfgName {
			continue
		}

		if e.Op&(inotify.Create|inotify.Write|inotify.Rename) == 0 {
			continue
		}

		log.Infof("Config has changed: reloading")
		cfg, err := loadConfig(*cfgFilepath)
		if err != nil {
			log.Errorf("unable to reload config: %v", err)
			reloadFailed.SetFailed()
			continue
		}

		err = pm.Configure(cfg)
		if err != nil {
			log.Fatalf("reconfiguration failed: %v", err)
		}

		reloadFailed.SetOK()
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
