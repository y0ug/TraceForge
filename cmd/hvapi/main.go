package main

import (
	"encoding/json"
	"fmt"
	"hvapi/pkg/hvlib"
	"hvapi/pkg/sblib/helpers"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type HttpResp struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Define a struct to hold provider instances
type ProviderRegistry struct {
	providers map[string]hvlib.VirtualizationProvider
	mu        sync.Mutex
}

// Initialize the provider registry
var providers = ProviderRegistry{
	providers: make(map[string]hvlib.VirtualizationProvider),
}

// Register a provider
func (pr *ProviderRegistry) RegisterProvider(name string, provider hvlib.VirtualizationProvider) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.providers[name] = provider
}

// Get a provider
func (pr *ProviderRegistry) GetProvider(name string) hvlib.VirtualizationProvider {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.providers[name]
}

type Server struct {
	*helpers.Server
	AuthToken string
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

	// if err := godotenv.Load(); err != nil {
	// 	logrus.Warning("Error loading .env file")
	// }
	//
	// expectedToken := os.Getenv("API_AUTH_TOKEN")

	configPath := "config.toml"
	configLoader, err := hvlib.NewConfigLoader(configPath)
	if err != nil {
		logger.Fatalf("Error loading configuration: %v", err)
	}

	vmware := initializeProvider(&hvlib.VmwareVP{}, configLoader, "VMware")
	hyperv := initializeProvider(&hvlib.HypervVP{}, configLoader, "Hyper-V")

	providers.RegisterProvider("vmware", vmware)
	providers.RegisterProvider("hyperv", hyperv)

	authToken := configLoader.GetString("api.auth_token")
	if authToken == "" {
		logrus.Fatal("api.auth_token is not set")
	}

	server := &Server{
		Server:    &utils.Server{Logger: logger},
		AuthToken: authToken,
	}

	// Create a new router
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/providers", server.listProvidersHandler).Methods("GET")
	router.HandleFunc("/{provider}/{vmname}/snapshots", server.snapshotsVMHandler).Methods("GET")
	router.HandleFunc("/{provider}", server.listVMsHandler).Methods("GET")
	router.HandleFunc("/{provider}/{vmname}/snapshot/{snapshotname}", server.basicVMHandler("snapshot")).Methods("GET")
	router.HandleFunc("/{provider}/{vmname}/snapshot/{snapshotname}", server.basicVMHandler("snapshot")).Methods("DELETE")
	router.Use(server.RequestLoggingMiddleware())
	router.Use(server.AuthMiddleware)

	// Define VM operation routes dynamically
	actions := []string{"start", "stop", "suspend", "revert", "reset"}
	for _, action := range actions {
		router.HandleFunc(fmt.Sprintf("/{provider}/{vmname}/%s", action), server.basicVMHandler(action)).Methods("GET")
	}

	// Start the server
	port := 8080
	logger.Infof("Server listening on :%d", port)
	if err := http.ListenAndServe(
		fmt.Sprintf(":%d", port), router); err != nil {
		logrus.Fatal(err)
	}
}

// Initializes a provider and loads VMs
func initializeProvider(provider hvlib.VirtualizationProvider, loader *hvlib.ConfigLoader, name string) hvlib.VirtualizationProvider {
	if err := provider.LoadVMs(loader); err != nil {
		log.Fatalf("Error loading %s VMs: %v", name, err)
	}
	return provider
}

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logrus.Warn("No Authorization header")
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			logrus.Warn("Invalid Authorization header format")
			writeErrorResponse(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		// Validate token
		if headerParts[1] != s.AuthToken {
			logrus.Warn("Invalid token")
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Token is valid, proceed with the request
		next.ServeHTTP(w, r)
	})
}

// Handler for listing providers
func (s *Server) listProvidersHandler(w http.ResponseWriter, r *http.Request) {
	providerNames := make([]string, 0, len(providers.providers))
	for name := range providers.providers {
		providerNames = append(providerNames, name)
	}
	writeSuccessResponse(w, "", providerNames)
}

// Handler for listing VMs
func (s *Server) listVMsHandler(w http.ResponseWriter, r *http.Request) {
	provider := getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vms, err := provider.List()
	if err != nil {
		writeErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeSuccessResponse(w, "", vms)
}

// Handler for VM snapshots
func (s *Server) snapshotsVMHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vmName := vars["vmname"]
	snapshots, err := provider.ListSnapshots(vmName)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if _, ok := err.(*hvlib.VmNotFoundError); ok {
			httpStatus = http.StatusNotFound
		}
		writeErrorResponse(w, err.Error(), httpStatus)
		return
	}
	writeSuccessResponse(w, "", snapshots)
}

// Generic handler for VM operations
func (s *Server) basicVMHandler(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		provider := getProviderFromRequest(w, r)
		if provider == nil {
			return
		}

		vmName := vars["vmname"]
		var err error

		// Perform action
		switch action {
		case "start":
			err = provider.Start(vmName)
		case "stop":
			err = provider.Stop(vmName, true)
		case "suspend":
			err = provider.Suspend(vmName)
		case "revert":
			err = provider.Revert(vmName)
		case "reset":
			err = provider.Reset(vmName)
		case "snapshot":
			snapshotName := vars["snapshotname"]
			if r.Method == "DELETE" {
				err = provider.DeleteSnapshot(vmName, snapshotName)
			} else {
				err = provider.TakeSnapshot(vmName, snapshotName)
			}
		default:
			writeErrorResponse(w, "invalid action", http.StatusBadRequest)
			return
		}

		if err != nil {
			httpStatus := http.StatusInternalServerError
			if _, ok := err.(*hvlib.VmNotFoundError); ok {
				httpStatus = http.StatusNotFound
			}
			writeErrorResponse(w, err.Error(), httpStatus)
			return
		}

		writeSuccessResponse(w,
			fmt.Sprintf("%s on %s completed successfully",
				action, vmName),
			nil)
	}
}

// Helper: Get provider from request
func getProviderFromRequest(w http.ResponseWriter, r *http.Request) hvlib.VirtualizationProvider {
	vars := mux.Vars(r)
	providerName := vars["provider"]

	provider := providers.GetProvider(providerName)
	if provider == nil {
		writeErrorResponse(w, "Provider not found", http.StatusNotFound)
		return nil
	}
	return provider
}

// Helper: Write JSON response

func writeJSONResponse(w http.ResponseWriter, httpStatus int, data HttpResp) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(data)
}

func writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	writeJSONResponse(w,
		http.StatusOK,
		HttpResp{Status: "success", Data: data, Message: message})
}

// Helper: Write error response
func writeErrorResponse(w http.ResponseWriter, message string, httpStatus int) {
	writeJSONResponse(w,
		httpStatus,
		HttpResp{Status: "error", Data: nil, Message: message})
}
