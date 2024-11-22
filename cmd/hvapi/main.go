package main

import (
	"TraceForge/internals/commons"
	"TraceForge/internals/hvapi"
	"TraceForge/pkg/hvlib"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func initRouter(server *hvapi.Server) *mux.Router {
	// Create a new router
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/providers", server.ListProvidersHandler).Methods("GET")
	router.HandleFunc("/{provider}/{vmname}/snapshots", server.SnapshotsVMHandler).Methods("GET")
	router.HandleFunc("/{provider}", server.ListVMsHandler).Methods("GET")
	router.HandleFunc("/{provider}/{vmname}/snapshot/{snapshotname}", server.BasicVMHandler("snapshot")).Methods("GET")
	router.HandleFunc("/{provider}/{vmname}/snapshot/{snapshotname}", server.BasicVMHandler("snapshot")).Methods("DELETE")

	// Define VM operation routes dynamically
	actions := []string{"start", "stop", "suspend", "revert", "reset"}
	for _, action := range actions {
		router.HandleFunc(fmt.Sprintf("/{provider}/{vmname}/%s", action), server.BasicVMHandler(action)).Methods("GET")
	}

	return router
}

func main() {
	// Set up logging
	logger := logrus.New()
	// logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	configPath := "config.toml"
	configLoader, err := hvlib.NewConfigLoader(configPath)
	if err != nil {
		logger.Fatalf("Error loading configuration: %v", err)
	}

	vmware := hvapi.InitializeProvider(&hvlib.VmwareVP{}, configLoader, "VMware")
	hyperv := hvapi.InitializeProvider(&hvlib.HypervVP{}, configLoader, "Hyper-V")

	providers := hvapi.NewProvider()
	providers.RegisterProvider("vmware", vmware)
	providers.RegisterProvider("hyperv", hyperv)

	authToken := configLoader.GetString("api.auth_token")
	if authToken == "" {
		logrus.Fatal("api.auth_token is not set")
	}

	server := &hvapi.Server{
		Server:    &commons.Server{Logger: logger},
		AuthToken: authToken,
		Providers: providers,
	}

	router := initRouter(server)
	router.Use(server.LoggingMiddleware())
	router.Use(server.AuthMiddleware)

	// Start the server
	port := 8080
	logger.Infof("Server listening on :%d", port)
	if err := http.ListenAndServe(
		fmt.Sprintf(":%d", port), router); err != nil {
		logrus.Fatal(err)
	}
}
