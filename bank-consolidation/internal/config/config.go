package config

import (
    "fmt"
    "os"
)

type Config struct {
    DBUser string
    DBPass string
    DBHost string
    DBPort string
    DBName string
    Addr   string
}

func getenv(k, def string) string {
    if v := os.Getenv(k); v != "" {
        return v
    }
    return def
}

func New() Config {
    return Config{
        DBUser: getenv("DB_USER", "root"),
        DBPass: getenv("DB_PASS", ""),
        DBHost: getenv("DB_HOST", "127.0.0.1"),
        DBPort: getenv("DB_PORT", "3306"),
        DBName: getenv("DB_NAME", "bank_consolidation"),
        Addr:   getenv("ADDR", ":8080"),
    }
}

func (c Config) MySQLDSN() string {
    if dsn := os.Getenv("READ_DSN"); dsn != "" {
        return dsn
    }
    auth := c.DBUser
    if c.DBPass != "" {
        auth += ":" + c.DBPass
    }
    return fmt.Sprintf("%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4,utf8&loc=Local", auth, c.DBHost, c.DBPort, c.DBName)
}
