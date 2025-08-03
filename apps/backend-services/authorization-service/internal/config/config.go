package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	PublicKeyURL string `mapstructure:"public_key_url"`
	Issuer       string `mapstructure:"issuer"`
	Audience     string `mapstructure:"audience"`
}

type KeycloakConfig struct {
	BaseURL      string `mapstructure:"base_url"`
	Realm        string `mapstructure:"realm"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/authorization-service")

	// Set defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)

	// Environment variable bindings
	viper.AutomaticEnv()
	viper.SetEnvPrefix("AUTHZ")

	// Bind environment variables
	viper.BindEnv("server.port", "AUTHZ_SERVER_PORT")
	viper.BindEnv("server.mode", "AUTHZ_SERVER_MODE")
	viper.BindEnv("database.host", "AUTHZ_DB_HOST")
	viper.BindEnv("database.port", "AUTHZ_DB_PORT")
	viper.BindEnv("database.user", "AUTHZ_DB_USER")
	viper.BindEnv("database.password", "AUTHZ_DB_PASSWORD")
	viper.BindEnv("database.dbname", "AUTHZ_DB_NAME")
	viper.BindEnv("redis.host", "AUTHZ_REDIS_HOST")
	viper.BindEnv("redis.port", "AUTHZ_REDIS_PORT")
	viper.BindEnv("redis.password", "AUTHZ_REDIS_PASSWORD")
	viper.BindEnv("jwt.public_key_url", "AUTHZ_JWT_PUBLIC_KEY_URL")
	viper.BindEnv("jwt.issuer", "AUTHZ_JWT_ISSUER")
	viper.BindEnv("jwt.audience", "AUTHZ_JWT_AUDIENCE")
	viper.BindEnv("keycloak.base_url", "AUTHZ_KEYCLOAK_BASE_URL")
	viper.BindEnv("keycloak.realm", "AUTHZ_KEYCLOAK_REALM")
	viper.BindEnv("keycloak.client_id", "AUTHZ_KEYCLOAK_CLIENT_ID")
	viper.BindEnv("keycloak.client_secret", "AUTHZ_KEYCLOAK_CLIENT_SECRET")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
