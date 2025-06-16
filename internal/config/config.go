package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Telegram    TelegramConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Solana      SolanaConfig
	Jupiter     JupiterConfig
	Jito        JitoConfig
	DexScreener DexScreenerConfig
	App         AppConfig
}

type TelegramConfig struct {
	BotToken string
	Debug    bool
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

type RedisConfig struct {
	URL string
}

type SolanaConfig struct {
	RPCURL string
}

type JupiterConfig struct {
	APIURL string
}

type JitoConfig struct {
	RPCURL string
	UUID   string
	Debug  bool
}

type DexScreenerConfig struct {
	APIURL string
}

type AppConfig struct {
	Debug bool
}

// LoadResult содержит результат загрузки конфигурации
type LoadResult struct {
	Config   *Config
	Errors   []error
	Warnings []string
}

// Load загружает конфигурацию из environment variables
func Load() *LoadResult {
	result := &LoadResult{
		Config:   &Config{},
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}

	// Показываем текущее окружение
	fmt.Printf("🌍 Environment: %s\n", getCurrentEnvironment())

	if err := godotenv.Load(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Warning: .env file not found or couldn't be loaded: %v", err))
	}

	result.loadTelegramConfig()

	result.loadDatabaseConfig()

	result.loadRedisConfig()

	result.loadSolanaConfig()

	result.loadJupiterConfig()

	result.loadJitoConfig()

	result.loadDexScreenerConfig()

	result.loadAppConfig()

	return result
}

func (r *LoadResult) loadTelegramConfig() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		r.Errors = append(r.Errors, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required"))
		return
	}

	r.Config.Telegram = TelegramConfig{
		BotToken: botToken,
		Debug:    getEnvBool("BOT_DEBUG", false),
	}
}

func (r *LoadResult) loadDatabaseConfig() {
	portStr := getEnvOrDefault("DB_PORT", "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Errorf("invalid DB_PORT value '%s': %v", portStr, err))
		return
	}

	r.Config.Database = DatabaseConfig{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     port,
		User:     getEnvOrDefault("DB_USER", "postgres"),
		Password: getEnvOrDefault("DB_PASSWORD", "password"),
		Database: getEnvOrDefault("DB_NAME", "cash_farmer"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
	}

	// Предупреждение для дефолтного пароля
	if r.Config.Database.Password == "password" {
		r.Warnings = append(r.Warnings, "Warning: using default database password 'password' - consider setting DB_PASSWORD")
	}
}

func (r *LoadResult) loadRedisConfig() {
	r.Config.Redis = RedisConfig{
		URL: getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
	}
}

func (r *LoadResult) loadSolanaConfig() {
	// НЕ используем дефолтное значение с API ключом для безопасности
	rpcURL := os.Getenv("SOLANA_RPC_URL")
	if rpcURL == "" {
		// Используем публичный RPC как fallback
		rpcURL = "https://api.mainnet-beta.solana.com"
		r.Warnings = append(r.Warnings, "Warning: SOLANA_RPC_URL not set, using public RPC endpoint (may be slow)")
	}

	r.Config.Solana = SolanaConfig{
		RPCURL: rpcURL,
	}
}

func (r *LoadResult) loadJupiterConfig() {
	r.Config.Jupiter = JupiterConfig{
		APIURL: getEnvOrDefault("JUPITER_API_URL", "https://quote-api.jup.ag/v6"),
	}
}

func (r *LoadResult) loadJitoConfig() {
	r.Config.Jito = JitoConfig{
		RPCURL: getEnvOrDefault("JITO_RPC_URL", "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1"),
		UUID:   os.Getenv("JITO_UUID"), // Опционально, без дефолтного значения
		Debug:  getEnvBool("JITO_DEBUG", false),
	}
}

func (r *LoadResult) loadDexScreenerConfig() {
	r.Config.DexScreener = DexScreenerConfig{
		APIURL: getEnvOrDefault("DEXSCREENER_API_URL", "https://api.dexscreener.com"),
	}
}

func (r *LoadResult) loadAppConfig() {
	r.Config.App = AppConfig{
		Debug: getEnvBool("APP_DEBUG", false),
	}
}

// HasErrors возвращает true если есть критические ошибки
func (r *LoadResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetErrorMessage возвращает объединённое сообщение об ошибках
func (r *LoadResult) GetErrorMessage() string {
	if len(r.Errors) == 0 {
		return ""
	}

	var messages []string
	for _, err := range r.Errors {
		messages = append(messages, err.Error())
	}

	return strings.Join(messages, "; ")
}

// PrintWarnings выводит все предупреждения
func (r *LoadResult) PrintWarnings() {
	for _, warning := range r.Warnings {
		fmt.Println(warning)
	}
}

// Утилиты для работы с environment variables

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultValue
	}
}

// isDevelopmentEnvironment определяет, является ли текущее окружение development
func isDevelopmentEnvironment() bool {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	// Development: пустое значение, "dev", "development", "local"
	// Production: "prod", "production"
	return env == "" || env == "dev" || env == "development" || env == "local"
}

// isProductionEnvironment определяет, является ли текущее окружение production
func isProductionEnvironment() bool {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	return env == "prod" || env == "production"
}

// getCurrentEnvironment возвращает строковое представление текущего окружения
func getCurrentEnvironment() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		return "development (default)"
	}
	return env
}
