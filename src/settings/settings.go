// Package settings handles loading and validating the `settings.yaml` configuration file.
// It ensures a default file is created if one is missing or incomplete, and allows querying settings using dot-separated keys (e.g., "logger.debug").
package settings

import (
	"os"
	"strings"
	"path/filepath"

	"atsuko-nexus/src/logger"
	"gopkg.in/yaml.v3"
)

// configMap holds the in-memory parsed YAML configuration data.
var configMap map[string]interface{}

// configFile stores the absolute path to the `settings.yaml` file.
var configFile string

// init is called automatically at startup. It resolves the config file path relative to the executable and loads the settings into memory.
// If the config is missing, invalid, or missing keys, it rewrites it with defaults.
func init() {
	exePath, err := os.Executable()
	if err != nil {
		panic("Failed to get executable path: " + err.Error())
	}
	exeDir := filepath.Dir(exePath)
	configFile = filepath.Join(exeDir, "settings.yaml")

	loadSettings()
}

// loadSettings loads and validates the YAML configuration file.
// It creates or rewrites the file if it’s missing, malformed, or missing required keys.
func loadSettings() {
	// Check if the config file exists
	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		logger.Log("WARNING", "settings", "settings.yaml not found. Creating default config.")
		writeDefault()
	}

	// Read the file content
	raw, err := os.ReadFile(configFile)
	if err != nil {
		logger.Log("ERROR", "settings", "Failed to read settings.yaml: "+err.Error())
		writeDefault()
		raw = []byte(defaultYAML)
	}

	// Parse YAML content into configMap
	err = yaml.Unmarshal(raw, &configMap)
	if err != nil {
		logger.Log("ERROR", "settings", "settings.yaml is not valid YAML. Overwriting with default.")
		writeDefault()
		raw = []byte(defaultYAML)
		yaml.Unmarshal(raw, &configMap)
	}

	// Compare keys to ensure all expected fields exist
	var defaultMap map[string]interface{}
	yaml.Unmarshal([]byte(defaultYAML), &defaultMap)
	if !validateKeys(defaultMap, configMap) {
		logger.Log("WARNING", "settings", "settings.yaml missing keys. Replacing with default.")
		writeDefault()
		yaml.Unmarshal([]byte(defaultYAML), &configMap)
	}

	logger.Log("INFO", "settings", "settings.yaml loaded successfully.")
}

// Get returns the value of a setting using dot-separated keys (e.g., "logger.debug").
// It traverses nested maps and returns nil if the key does not exist.
func Get(key string) interface{} {
	keys := strings.Split(key, ".")
	var current any = configMap
	for _, k := range keys {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[k]
		} else {
			return nil
		}
	}
	return current
}

// validateKeys recursively checks whether all keys from defaultMap exist in targetMap.
// This ensures compatibility if new config fields are added in future updates.
func validateKeys(defaultMap, targetMap map[string]interface{}) bool {
	for k, v := range defaultMap {
		val, ok := targetMap[k]
		if !ok {
			return false
		}
		if sub, isMap := v.(map[string]interface{}); isMap {
			if subVal, isSubMap := val.(map[string]interface{}); isSubMap {
				if !validateKeys(sub, subVal) {
					return false
				}
			} else {
				return false
			}
		}
	}
	return true
}

// writeDefault writes the defaultYAML content to `settings.yaml` on disk.
// It is called when the config is missing or needs to be replaced.
func writeDefault() {
	err := os.WriteFile(configFile, []byte(defaultYAML), 0644)
	if err != nil {
		logger.Log("ERROR", "settings", "Failed to write default settings.yaml: "+err.Error())
	}
}

// defaultYAML is the full default config as a string
const defaultYAML = `# === LOGGER CONFIGURATION ===
logger:
  debug: false
  info: true
  warning: true
  caution: true
  error: true
  log_to_file: true
  log_file_path: "./logs/runtime.log"
  rotate_logs: true
  max_log_size_mb: 10
  max_log_age_days: 7

# === UI SETTINGS ===
ui:
  show_peer_count: true
  panel_refresh_time: 1
  theme: "default"

# === HEARTBEAT & METRICS ===
metrics:
  heartbeat_interval: 120
  enable_metrics: true
  cpu_monitoring: true
  ram_monitoring: true
  net_traffic_monitoring: true

# === NETWORK CONFIGURATION ===
network:
  listen_port: 51613
  enable_upnp: true
  bind_address: "0.0.0.0"
  peer_discovery_interval: 60
  max_peers: 100
  reconnect_attempts: 5
  reconnect_interval: 15
  enable_nat_traversal: true
  allow_lan_peers: true

# === PEER TRUST & IDENTITY ===
identity:
  admin_key: "none"
  require_signed_peers: false

# === STORAGE & PERSISTENCE ===
storage:
  data_dir: "./data"
  peer_cache_file: "./data/peers/peers.yaml"

# === TASK PROCESSING ===
tasks:
  enable_task_queue: false
  max_concurrent_tasks: 5
  task_timeout_sec: 120
  job_blacklist:
    - "malicious"
    - "spam"

# === API / WEB INTERFACE ===
api:
  enable_rest_api: false
  rest_api_port: 9090
  enable_web_ui: false
  web_ui_port: 9091
  require_api_auth: true
  api_token: "change_me"

# === RATE LIMITING ===
limits:
  rate_limit_per_minute: 60
  max_messages_per_peer: 100
  cooldown_on_limit_hit: 10
`