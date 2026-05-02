// Simple app with health and readiness checks,
// and Prometheus metrics for Kubernetes testing
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed favicon.ico
var faviconContent []byte

// Release is the release tag injected via -ldflags.
var Release string

// GitCommit is the git commit hash injected via -ldflags.
var GitCommit string

// BuildTime is the build timestamp injected via -ldflags.
var BuildTime string

var counter int
var port = 8081
var mport int
var mutex = &sync.Mutex{}

var readyDelay = 10

// Prometheus Counter var
var (
	curlErrorCollector = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "error_curl_total",
			Help: "Total curl request failed",
		},
		[]string{"vendor"},
	)
)

// Prometheus Summary var
var (
	httpRequestsResponseTime = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: "http",
			Name:      "response_time_seconds",
			Help:      "Request response times",
		},
	)
)

// PROMETHEUS CUSTOM METRIC
// Annotate the K8S Pods so Prometheus starts scraping
//
//	annotations:
//	  prometheus.io/scrape: "true"
//	  prometheus.io/port: "8081"
//	  prometheus.io/path: "/metrics" # this is the default
func init() {
	prometheus.MustRegister(curlErrorCollector)
	prometheus.MustRegister(httpRequestsResponseTime)
}

// sanitizeVendor cleans vendor label values to prevent unbounded Prometheus cardinality.
// Only alphanumeric characters, hyphens and underscores are kept; max 64 chars.
func sanitizeVendor(v string) string {
	if v == "" {
		return "unknown"
	}
	const maxLen = 64
	var b strings.Builder
	for _, c := range v {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteRune(c)
		}
		if b.Len() >= maxLen {
			break
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// checkRest: Rest api handler
func checkRest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	vendor := sanitizeVendor(r.FormValue("vendor"))

	// Simulate random failure
	failed := rand.Intn(2) == 0
	if failed {
		RecordCurlError(vendor)
		http.Error(w, "Failed to fetch", http.StatusBadGateway)
	} else {
		fmt.Fprintln(w, "Vendor status: ok")
	}

	httpRequestsResponseTime.Observe(float64(time.Since(start).Seconds()))
}

// RecordCurlError increments the error counter for the given vendor.
func RecordCurlError(vendor string) {
	curlErrorCollector.With(prometheus.Labels{"vendor": vendor}).Inc()
}

// HANDLERS //
// echoString: Echo handler
func echoString(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	name, err := os.Hostname()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r.URL.Path != "/" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Create random delay
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	delay := int32(rnd.Float64() * 1000)
	time.Sleep(time.Millisecond * time.Duration(delay))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "I am: "+name)

	mutex.Lock()
	counter++
	fmt.Fprintln(w, "Requests: "+strconv.Itoa(counter))
	mutex.Unlock()

	fmt.Fprintf(w, "Time: %s\n", time.Now().Format(time.RFC1123))

	// Update the metrics
	httpRequestsResponseTime.Observe(float64(time.Since(start).Seconds()))
}

// healthz: Health handler
func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

// readyz: Readiness handler
func readyz(w http.ResponseWriter, r *http.Request, isReady *atomic.Value) {
	if isReady == nil || !isReady.Load().(bool) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

// faviconHandler serves the embedded favicon.
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Write(faviconContent)
}

// versionHandler returns build metadata as JSON.
func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Release   string `json:"release"`
		GitCommit string `json:"git_commit"`
		BuildTime string `json:"build_time"`
	}{Release, GitCommit, BuildTime})
}

func main() {
	// Change the port via env var
	penv := os.Getenv("PORT")
	if penv != "" {
		eport, err := strconv.Atoi(penv)
		if err != nil {
			log.Fatalf("invalid PORT value %q: %v", penv, err)
		}
		port = eport
	}
	// Optional metrics port via env var
	metricsPort, prometheusEnabled := os.LookupEnv("METRICS_PORT")
	if prometheusEnabled {
		meport, err := strconv.Atoi(metricsPort)
		if err != nil {
			log.Fatalf("invalid METRICS_PORT value %q: %v", metricsPort, err)
		}
		mport = meport
	}
	// Optional readiness delay via env var
	rdEnv := os.Getenv("READY_DELAY")
	if rdEnv != "" {
		rd, err := strconv.Atoi(rdEnv)
		if err != nil {
			log.Fatalf("invalid READY_DELAY value %q: %v", rdEnv, err)
		}
		readyDelay = rd
	}

	// CLI flags override env vars
	pflag := flag.String("port", "", "service port")
	mpflag := flag.String("metrics_port", "", "metrics port")
	rdflag := flag.String("ready_delay", "", "readiness delay in seconds")
	vflag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *vflag {
		fmt.Printf("release=%s commit=%s built=%s\n", Release, GitCommit, BuildTime)
		os.Exit(0)
	}

	if *pflag != "" {
		cport, err := strconv.Atoi(*pflag)
		if err != nil {
			log.Fatalf("invalid -port value %q: %v", *pflag, err)
		}
		port = cport
	}
	if *mpflag != "" {
		mcport, err := strconv.Atoi(*mpflag)
		if err != nil {
			log.Fatalf("invalid -metrics_port value %q: %v", *mpflag, err)
		}
		mport = mcport
	}
	if *rdflag != "" {
		rd, err := strconv.Atoi(*rdflag)
		if err != nil {
			log.Fatalf("invalid -ready_delay value %q: %v", *rdflag, err)
		}
		readyDelay = rd
	}

	sport := ":" + strconv.Itoa(port)
	smport := ":" + strconv.Itoa(mport)

	httpMux := http.NewServeMux()

	// Create server instance
	s := &http.Server{
		Addr:           sport,
		Handler:        httpMux,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   5 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // Max header of 1MB
	}

	// Handle metrics on the http port
	if mport == 0 || mport == port {
		httpMux.Handle("/metrics", promhttp.Handler())
	}

	// Handle metrics on a separate metrics port
	var metricsServer *http.Server
	if mport > 0 && mport != port {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		metricsServer = &http.Server{
			Addr:         smport,
			Handler:      metricsMux,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 5 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
		log.Print("Starting prometheus service listening on port " + smport + " ...")
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal(err)
			}
		}()
	}

	// hit this api several time with query string vendor=something
	httpMux.HandleFunc("/checkrest", checkRest)
	httpMux.HandleFunc("/version", versionHandler)

	httpMux.HandleFunc("/", echoString)
	httpMux.HandleFunc("/favicon.ico", faviconHandler)

	// Liveness probe
	httpMux.HandleFunc("/healthz", healthz)

	// Readiness probe (simulate startup delay)
	isReady := &atomic.Value{}
	isReady.Store(false)
	go func() {
		log.Printf("Ready NOK")
		time.Sleep(time.Duration(readyDelay) * time.Second)
		isReady.Store(true)
		log.Printf("Ready OK")
	}()
	httpMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		readyz(w, r, isReady)
	})

	// Start server in background
	go func() {
		log.Print("Starting the service listening on port " + sport + " ...")
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	fmt.Println("\rCaught signal...shutting down.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatal("Server shutdown error:", err)
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Fatal("Metrics server shutdown error:", err)
		}
	}
	log.Println("Server exited.")
}
