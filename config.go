package clauderelay

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	Port         string            `json:"port"`
	ClaudePath   string            `json:"claude_path"`
	MaxProcesses int               `json:"max_processes"`
	Endpoints    map[string]string `json:"endpoints,omitempty"`
	AllowOrigins []string          `json:"allow_origins,omitempty"`
	TempDirBase  string            `json:"temp_dir_base,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	config := &Config{
		Port:         "8080",
		ClaudePath:   "claude",
		MaxProcesses: 100,
		Endpoints:    make(map[string]string),
		AllowOrigins: []string{"*"},
		TempDirBase:  os.TempDir(),
	}

	if path == "" {
		return config, nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

func DefaultConfig() *Config {
	return &Config{
		Port:         "8080",
		ClaudePath:   "claude",
		MaxProcesses: 100,
		Endpoints: map[string]string{
			"/ws": "default",
		},
		AllowOrigins: []string{"*"},
		TempDirBase:  os.TempDir(),
	}
}