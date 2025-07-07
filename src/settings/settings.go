package settings

import (
	"os"
	"strings"

	"atsuko-nexus/src/logger"
	"gopkg.in/yaml.v3"
)

var configMap map[string]interface{}

const configFile = "./settings.yaml"

func init() {
	// Check existence
	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		logger.Log("WARNING", "settings", "settings.yaml not found. Creating default config.")
		writeDefault()
	}

	// Load and parse file
	raw, err := os.ReadFile(configFile)
	if err != nil {
		logger.Log("ERROR", "settings", "Failed to read settings.yaml: "+err.Error())
		writeDefault()
		raw = []byte(defaultYAML)
	}

	err = yaml.Unmarshal(raw, &configMap)
	if err != nil {
		logger.Log("ERROR", "settings", "settings.yaml is not valid YAML. Overwriting with default.")
		writeDefault()
		raw = []byte(defaultYAML)
		yaml.Unmarshal(raw, &configMap)
	}

	// Validate config keys
	var defaultMap map[string]interface{}
	yaml.Unmarshal([]byte(defaultYAML), &defaultMap)
	if !validateKeys(defaultMap, configMap) {
		logger.Log("WARNING", "settings", "settings.yaml missing keys. Replacing with default.")
		writeDefault()
		yaml.Unmarshal([]byte(defaultYAML), &configMap)
	}

	logger.Log("INFO", "settings", "settings.yaml loaded successfully.")
}

// Get returns a setting value by dot-separated key like "logger.debug"
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

// validateKeys ensures all keys in defaultMap exist in targetMap
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

// writeDefault saves the default config to disk
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
  listen_port: 8080
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