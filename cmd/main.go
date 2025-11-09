package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"git.oxl.at/micro-queue/internal/config"
	"git.oxl.at/micro-queue/internal/queue"
	"git.oxl.at/micro-queue/internal/server"
	"git.oxl.at/micro-queue/internal/u"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func welcome() {
	fmt.Printf("\n __       __  __                                       ______                                          \n")
	fmt.Println("/  \\     /  |/  |                                     /      \\                                         ")
	fmt.Println("$$  \\   /$$ |$$/   _______   ______    ______        /$$$$$$  | __    __   ______   __    __   ______  ")
	fmt.Println("$$$  \\ /$$$ |/  | /       | /      \\  /      \\       $$ |  $$ |/  |  /  | /      \\ /  |  /  | /      \\ ")
	fmt.Println("$$$$  /$$$$ |$$ |/$$$$$$$/ /$$$$$$  |/$$$$$$  |      $$ |  $$ |$$ |  $$ |/$$$$$$  |$$ |  $$ |/$$$$$$  |")
	fmt.Println("$$ $$ $$/$$ |$$ |$$ |      $$ |  $$/ $$ |  $$ |      $$ |_ $$ |$$ |  $$ |$$    $$ |$$ |  $$ |$$    $$ |")
	fmt.Println("$$ |$$$/ $$ |$$ |$$ \\_____ $$ |      $$ \\__$$ |      $$ / \\$$ |$$ \\__$$ |$$$$$$$$/ $$ \\__$$ |$$$$$$$$/ ")
	fmt.Println("$$ | $/  $$ |$$ |$$       |$$ |      $$    $$/       $$ $$ $$< $$    $$/ $$       |$$    $$/ $$       |")
	fmt.Println("$$/      $$/ $$/  $$$$$$$/ $$/        $$$$$$/         $$$$$$  | $$$$$$/   $$$$$$$/  $$$$$$/   $$$$$$$/ ")
	fmt.Println("                                                          $$$/                                         ")
	fmt.Printf("Version: %v\n", config.VERSION)
	fmt.Printf("by OXL IT Services - https://git.oxl.at/micro-queue (License: MIT)\n\n")
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "", "Path to the config.yaml file")
	flag.Parse()

	welcome()

	// load config
	if configPath == "" {
		log.Fatalf("FATAL: You need to supply the path to the config-file")
	}

	appConfig, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}
	u.LogDebug(fmt.Sprintf("Successfully loaded config from %s", configPath))

	// init queues
	queueMap := make(map[string]*queue.PersistentQueue)
	var queueNames []string
	for _, qCfg := range appConfig.Queues {
		dirPath := filepath.Join(appConfig.Settings.Path, qCfg.Name)

		log.Printf("Initializing queue '%s' of kind %s in %s...", qCfg.Name, qCfg.Kind, dirPath)
		pq, err := queue.NewPersistentQueue(dirPath)
		if err != nil {
			log.Fatalf("Failed to initialize persistent queue '%s': %v", qCfg.Name, err)
		}
		queueMap[qCfg.Name] = pq
		queueNames = append(queueNames, qCfg.Name)
	}
	u.LogDebug(fmt.Sprintf("Successfully initialized %d queues: %s", len(queueMap), strings.Join(queueNames, ", ")))

	// server
	mux := http.NewServeMux()
	apiHandler := server.RootHandler(queueMap, appConfig)
	authHandler := server.AuthMiddleware(apiHandler, appConfig)
	finalHandler := server.ServerHeaderMiddleware(authHandler)

	mux.Handle("/", finalHandler)
	if appConfig.Settings.MetricExporter {
		mux.Handle("/metrics", promhttp.Handler())
	}

	listenStr := fmt.Sprintf("%v:%v", appConfig.Settings.ListenAddr, appConfig.Settings.ListenPort)
	log.Printf("Listening on http://%v", listenStr)
	if appConfig.Settings.MetricExporter {
		log.Printf("Metrics available at http://%v/metrics", listenStr)
	}
	log.Fatal(http.ListenAndServe(listenStr, mux))
}
