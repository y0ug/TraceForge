package main

import (
	"TraceForge/cmd/hvapi/docs"
	"TraceForge/internals/commons"
	"TraceForge/internals/hvapi"
	"TraceForge/pkg/hvlib"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger" // Swagger middleware
)

// @title Hypervisor API
// @version 1.0
// @description API for managing virtual machines across different hypervisors.
// @termsOfService http://example.com/terms/

// @contact.name API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func initRouter(server *hvapi.Server) *mux.Router {
	// Create a new router
	router := mux.NewRouter()

	// Create a subrouter for the Swagger route (no middleware applied)
	swaggerRouter := router.PathPrefix("/swagger/").Subrouter()
	swaggerRouter.PathPrefix("/").Handler(httpSwagger.WrapHandler)

	// Create a subrouter for API routes (middleware applied)
	apiRouter := router.PathPrefix("/").Subrouter()

	// Define routes
	apiRouter.HandleFunc("/providers", server.ListProvidersHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}/{vmname}/snapshots", server.SnapshotsVMHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}", server.ListVMsHandler).Methods("GET")

	apiRouter.HandleFunc("/{provider}/{vmname}/snapshot/{snapshotname}", server.TakeSnapshotHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}/{vmname}/snapshot/{snapshotname}", server.DeleteSnapshotHandler).Methods("DELETE")
	apiRouter.HandleFunc("/{provider}/{vmname}/start", server.StartVMHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}/{vmname}/stop", server.StopVMHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}/{vmname}/suspend", server.SuspendVMHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}/{vmname}/revert", server.RevertVMHandler).Methods("GET")
	apiRouter.HandleFunc("/{provider}/{vmname}/reset", server.ResetVMHandler).Methods("GET")

	apiRouter.Use(server.LoggingMiddleware())
	apiRouter.Use(server.AuthMiddleware)
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

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.toml"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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

	// Start the server
	listenOn := fmt.Sprintf(":%s", port)
	docs.SwaggerInfo.Host = fmt.Sprintf("127.0.0.1%s", listenOn)
	logger.Infof("Server listening on %s", listenOn)
	if err := http.ListenAndServe(
		listenOn, router); err != nil {
		logger.Fatal(err)
	}
}
