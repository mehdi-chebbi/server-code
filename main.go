package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type CreateServerRequest struct {
	UserID   string `json:"user_id"`
	Password string `json:"password,omitempty"`
}

type CreateServerResponse struct {
	Message        string `json:"message"`
	ReleaseName    string `json:"release_name"`
	Namespace      string `json:"namespace"`
	Password       string `json:"password"`
	PortForward    string `json:"port_forward_command"`
	AccessURL      string `json:"access_url"`
	HelmStatus     string `json:"helm_status"`
}

type DeleteServerRequest struct {
	UserID string `json:"user_id"`
}

const (
	codeServerRepoDir = "/opt/code-server"
	helmChartPath     = "ci/helm-chart"
)

func generatePassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}
	return string(output), nil
}

func installHelmRelease(userID, password string) (*CreateServerResponse, error) {
	releaseName := fmt.Sprintf("code-server-%s", userID)
	namespace := fmt.Sprintf("code-server-%s", userID)
	chartPath := filepath.Join(codeServerRepoDir, helmChartPath)
	
	log.Printf("Installing Helm release: %s in namespace: %s", releaseName, namespace)
	
	// Prepare Helm command with custom values
	args := []string{
		"upgrade", "--install",
		releaseName,
		chartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--set", fmt.Sprintf("password=%s", password),
		"--set", "persistence.enabled=true",
		"--set", "persistence.size=10Gi",
		"--wait",
		"--timeout", "5m",
	}
	
	output, err := runCommand("helm", args...)
	if err != nil {
		return nil, fmt.Errorf("helm install failed: %v, output: %s", err, output)
	}
	
	return &CreateServerResponse{
		Message:     "Code server deployed successfully via Helm",
		ReleaseName: releaseName,
		Namespace:   namespace,
		Password:    password,
		PortForward: fmt.Sprintf("kubectl port-forward --namespace %s service/%s 8080:http", namespace, releaseName),
		AccessURL:   "http://127.0.0.1:8080",
		HelmStatus:  "deployed",
	}, nil
}

func uninstallHelmRelease(userID string) error {
	releaseName := fmt.Sprintf("code-server-%s", userID)
	namespace := fmt.Sprintf("code-server-%s", userID)
	
	log.Printf("Uninstalling Helm release: %s from namespace: %s", releaseName, namespace)
	
	// Uninstall Helm release
	output, err := runCommand("helm", "uninstall", releaseName, "--namespace", namespace)
	if err != nil {
		log.Printf("Warning: helm uninstall failed: %v, output: %s", err, output)
	}
	
	// Delete namespace (this will clean up all resources)
	output, err = runCommand("kubectl", "delete", "namespace", namespace, "--ignore-not-found=true", "--wait=false")
	if err != nil {
		log.Printf("Warning: namespace deletion failed: %v, output: %s", err, output)
	}
	
	return nil
}

func listHelmReleases() ([]map[string]string, error) {
	// List all Helm releases with code-server prefix
	output, err := runCommand("helm", "list", "--all-namespaces", "--filter", "code-server-.*", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list helm releases: %v", err)
	}
	
	var releases []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &releases); err != nil {
		return nil, fmt.Errorf("failed to parse helm output: %v", err)
	}
	
	result := make([]map[string]string, 0)
	for _, release := range releases {
		result = append(result, map[string]string{
			"release_name": fmt.Sprintf("%v", release["name"]),
			"namespace":    fmt.Sprintf("%v", release["namespace"]),
			"status":       fmt.Sprintf("%v", release["status"]),
			"chart":        fmt.Sprintf("%v", release["chart"]),
		})
	}
	
	return result, nil
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		req.Password = generatePassword()
	}

	// Install Helm chart (repo already cloned in container)
	resp, err := installHelmRelease(req.UserID, req.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to install Helm chart: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeleteServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if err := uninstallHelmRelease(req.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Code server uninstalled successfully",
		"user_id": req.UserID,
	})
}

func handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	releases, err := listHelmReleases()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"releases": releases,
		"count":    len(releases),
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	// Verify helm, kubectl are available and repo exists
	_, helmErr := exec.LookPath("helm")
	_, kubectlErr := exec.LookPath("kubectl")
	
	status := "healthy"
	repoExists := false
	
	if _, err := os.Stat(filepath.Join(codeServerRepoDir, helmChartPath)); err == nil {
		repoExists = true
	}
	
	if helmErr != nil || kubectlErr != nil || !repoExists {
		status = "unhealthy"
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":              status,
		"helm_available":      helmErr == nil,
		"kubectl_available":   kubectlErr == nil,
		"repo_cloned":         repoExists,
		"chart_path":          filepath.Join(codeServerRepoDir, helmChartPath),
	})
}

func main() {
	// Verify the repo is cloned
	chartPath := filepath.Join(codeServerRepoDir, helmChartPath)
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		log.Fatalf("ERROR: Code-server repository not found at %s. The repo should be cloned during container build.", codeServerRepoDir)
	}
	
	log.Printf("Code-server repository found at: %s", codeServerRepoDir)
	log.Printf("Helm chart path: %s", chartPath)
	
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/create", handleCreate)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/list", handleList)

	log.Println("Code Server Controller starting on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}