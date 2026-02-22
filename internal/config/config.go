package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	// 服务器配置
	Server ServerConfig `yaml:"server"`

	// 搜索引擎配置
	Search SearchConfig `yaml:"search"`

	// 代理配置
	Proxy ProxyConfig `yaml:"proxy"`

	// MCP 配置
	MCP MCPConfig `yaml:"mcp"`

	// 浏览器配置
	Browser BrowserConfig `yaml:"browser"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port       int        `yaml:"port"`
	Host       string     `yaml:"host"`
	CORS       CORSConfig `yaml:"cors"`
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled bool   `yaml:"enabled"`
	Origin  string `yaml:"origin"`
}

// SearchConfig 搜索引擎配置
type SearchConfig struct {
	DefaultEngine  string   `yaml:"default_engine"`
	AllowedEngines []string `yaml:"allowed_engines"`
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

// MCPConfig MCP 协议配置
type MCPConfig struct {
	// 服务器信息
	ServerName    string `yaml:"server_name"`
	ServerVersion string `yaml:"server_version"`

	// 工具名称配置
	Tools MCPToolsConfig `yaml:"tools"`
}

// MCPToolsConfig MCP 工具名称配置
type MCPToolsConfig struct {
	SearchName        string `yaml:"search_name"`
	SearchDescription string `yaml:"search_description"`
}

// BrowserConfig 浏览器配置
type BrowserConfig struct {
	Enabled  bool `yaml:"enabled"`
	Headless bool `yaml:"headless"`
}

// ValidEngines 有效的搜索引擎列表
var ValidEngines = []string{"bing", "baidu", "duckduckgo", "google", "sogou", "browser_bing", "browser_baidu", "browser_google"}

// DefaultConfig 默认配置
var DefaultConfig = &Config{
	Server: ServerConfig{
		Port: 3456,
		Host: "0.0.0.0",
		CORS: CORSConfig{
			Enabled: false,
			Origin:  "*",
		},
	},
	Search: SearchConfig{
		DefaultEngine:  "duckduckgo",
		AllowedEngines: []string{},
	},
	Proxy: ProxyConfig{
		Enabled: false,
		URL:     "http://127.0.0.1:7890",
	},
	MCP: MCPConfig{
		ServerName:    "go-web-search-mcp",
		ServerVersion: "1.0.0",
		Tools: MCPToolsConfig{
			SearchName:        "search",
			SearchDescription: "Search the web using multiple engines (e.g., Bing, Baidu, DuckDuckGo) with no API key required. Returns structured results with title, URL, description, and source.",
		},
	},
	Browser: BrowserConfig{
		Enabled:  true,
		Headless: true,
	},
}

// configSearchPaths 配置文件搜索路径
var configSearchPaths = []string{
	"config.yaml",
	"config.yml",
	"configs/config.yaml",
	"configs/config.yml",
}

// Load 从 YAML 配置文件加载配置
// 支持通过 CONFIG_FILE 环境变量指定配置文件路径
func Load() *Config {
	// 复制默认配置
	cfg := *DefaultConfig

	// 查找配置文件
	configPath := findConfigFile()
	if configPath == "" {
		log.Printf("⚠️ No config file found, using default configuration")
		log.Printf("💡 You can create a config.yaml file or set CONFIG_FILE environment variable")
		cfg.validate()
		cfg.Print()
		return &cfg
	}

	// 读取配置文件
	log.Printf("📄 Loading configuration from: %s", configPath)
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("⚠️ Failed to read config file: %v, using defaults", err)
		cfg.validate()
		cfg.Print()
		return &cfg
	}

	// 解析 YAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("⚠️ Failed to parse config file: %v, using defaults", err)
		cfg.validate()
		cfg.Print()
		return &cfg
	}

	// 验证配置
	cfg.validate()

	// 打印配置信息
	cfg.Print()

	return &cfg
}

// LoadFromFile 从指定路径加载配置
func LoadFromFile(path string) (*Config, error) {
	cfg := *DefaultConfig

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file failed: %w", err)
	}

	cfg.validate()
	return &cfg, nil
}

// findConfigFile 查找配置文件
func findConfigFile() string {
	// 优先使用环境变量指定的配置文件
	if envPath := os.Getenv("CONFIG_FILE"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
		log.Printf("⚠️ CONFIG_FILE=%s not found, searching default paths", envPath)
	}

	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	var execDir string
	if err == nil {
		execDir = filepath.Dir(execPath)
	}

	// 获取当前工作目录
	workDir, _ := os.Getwd()

	// 搜索配置文件
	searchDirs := []string{workDir}
	if execDir != "" && execDir != workDir {
		searchDirs = append(searchDirs, execDir)
	}

	for _, dir := range searchDirs {
		for _, name := range configSearchPaths {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	return ""
}

// validate 验证并修正配置
func (c *Config) validate() {
	// 验证端口
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		log.Printf("⚠️ Invalid port %d, using default %d", c.Server.Port, DefaultConfig.Server.Port)
		c.Server.Port = DefaultConfig.Server.Port
	}

	// 验证 Host
	if c.Server.Host == "" {
		c.Server.Host = DefaultConfig.Server.Host
	}

	// 验证 CORS Origin
	if c.Server.CORS.Origin == "" {
		c.Server.CORS.Origin = DefaultConfig.Server.CORS.Origin
	}

	// 验证默认搜索引擎
	if !isValidEngine(c.Search.DefaultEngine) {
		log.Printf("⚠️ Invalid default_engine: %s, falling back to %s", c.Search.DefaultEngine, DefaultConfig.Search.DefaultEngine)
		c.Search.DefaultEngine = DefaultConfig.Search.DefaultEngine
	}

	// 验证允许的搜索引擎列表
	validAllowed := []string{}
	for _, e := range c.Search.AllowedEngines {
		e = strings.TrimSpace(e)
		if isValidEngine(e) {
			validAllowed = append(validAllowed, e)
		} else {
			log.Printf("⚠️ Invalid search engine ignored: %s", e)
		}
	}
	c.Search.AllowedEngines = validAllowed

	// 如果设置了允许列表，检查默认引擎是否在列表中
	if len(c.Search.AllowedEngines) > 0 && !contains(c.Search.AllowedEngines, c.Search.DefaultEngine) {
		log.Printf("⚠️ Default engine %s not in allowed list, using %s", c.Search.DefaultEngine, c.Search.AllowedEngines[0])
		c.Search.DefaultEngine = c.Search.AllowedEngines[0]
	}

	// 验证代理 URL
	if c.Proxy.Enabled && c.Proxy.URL == "" {
		log.Printf("⚠️ Proxy enabled but URL is empty, using default")
		c.Proxy.URL = DefaultConfig.Proxy.URL
	}

	// 验证 MCP 配置
	if c.MCP.ServerName == "" {
		c.MCP.ServerName = DefaultConfig.MCP.ServerName
	}
	if c.MCP.ServerVersion == "" {
		c.MCP.ServerVersion = DefaultConfig.MCP.ServerVersion
	}
	if c.MCP.Tools.SearchName == "" {
		c.MCP.Tools.SearchName = DefaultConfig.MCP.Tools.SearchName
	}
	if c.MCP.Tools.SearchDescription == "" {
		c.MCP.Tools.SearchDescription = DefaultConfig.MCP.Tools.SearchDescription
	}
}

// Print 打印配置信息
func (c *Config) Print() {
	log.Printf("🔍 Default search engine: %s", c.Search.DefaultEngine)
	if len(c.Search.AllowedEngines) > 0 {
		log.Printf("🔍 Allowed search engines: %s", strings.Join(c.Search.AllowedEngines, ", "))
	} else {
		log.Printf("🔍 No search engine restrictions, all available engines can be used")
	}
	if c.Proxy.Enabled {
		log.Printf("🌐 Using proxy: %s", c.Proxy.URL)
	} else {
		log.Printf("🌐 No proxy configured")
	}
	if c.Server.CORS.Enabled {
		log.Printf("🔒 CORS enabled with origin: %s", c.Server.CORS.Origin)
	} else {
		log.Printf("🔒 CORS disabled")
	}
	log.Printf("🔧 MCP Server: %s v%s", c.MCP.ServerName, c.MCP.ServerVersion)
	log.Printf("🔧 MCP Search tool name: %s", c.MCP.Tools.SearchName)
	log.Printf("🖥️ Server will listen on %s:%d", c.Server.Host, c.Server.Port)
}

// IsEngineAllowed 检查搜索引擎是否被允许使用
func (c *Config) IsEngineAllowed(engine string) bool {
	if len(c.Search.AllowedEngines) == 0 {
		return isValidEngine(engine)
	}
	return contains(c.Search.AllowedEngines, engine)
}

// 兼容性方法 - 为了减少对其他模块的修改

// GetPort 获取端口
func (c *Config) GetPort() int {
	return c.Server.Port
}

// GetHost 获取主机
func (c *Config) GetHost() string {
	return c.Server.Host
}

// IsEnableCORS 是否启用 CORS
func (c *Config) IsEnableCORS() bool {
	return c.Server.CORS.Enabled
}

// GetCORSOrigin 获取 CORS Origin
func (c *Config) GetCORSOrigin() string {
	return c.Server.CORS.Origin
}

// GetDefaultSearchEngine 获取默认搜索引擎
func (c *Config) GetDefaultSearchEngine() string {
	return c.Search.DefaultEngine
}

// IsUseProxy 是否使用代理
func (c *Config) IsUseProxy() bool {
	return c.Proxy.Enabled
}

// GetProxyURL 获取代理 URL
func (c *Config) GetProxyURL() string {
	return c.Proxy.URL
}

// GetMCPServerName 获取 MCP 服务器名称
func (c *Config) GetMCPServerName() string {
	return c.MCP.ServerName
}

// GetMCPServerVersion 获取 MCP 服务器版本
func (c *Config) GetMCPServerVersion() string {
	return c.MCP.ServerVersion
}

// GetMCPSearchToolName 获取 MCP 搜索工具名称
func (c *Config) GetMCPSearchToolName() string {
	return c.MCP.Tools.SearchName
}

// GetMCPSearchToolDescription 获取 MCP 搜索工具描述
func (c *Config) GetMCPSearchToolDescription() string {
	return c.MCP.Tools.SearchDescription
}

// IsBrowserEnabled 是否启用浏览器引擎
func (c *Config) IsBrowserEnabled() bool {
	return c.Browser.Enabled
}

// IsBrowserHeadless 浏览器是否使用无头模式
func (c *Config) IsBrowserHeadless() bool {
	return c.Browser.Headless
}

func isValidEngine(engine string) bool {
	return contains(ValidEngines, engine)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
