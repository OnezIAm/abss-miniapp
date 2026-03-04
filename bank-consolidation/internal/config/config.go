package config

import (
    "fmt"
    "os"
    "path/filepath"
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
        Addr:   getenv("ADDR", ":8585"),
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

func ResolveDataDir() string {
    if dir := os.Getenv("APP_DATA_DIR"); dir != "" {
        return dir
    }

    if path := os.Getenv("SQLITE_PATH"); path != "" {
        return filepath.Dir(path)
    }

    if cwd, err := os.Getwd(); err == nil {
        if fileExists(filepath.Join(cwd, "go.mod")) {
            return filepath.Join(cwd, "data")
        }
    }

    if dir, err := os.UserConfigDir(); err == nil && dir != "" {
        return filepath.Join(dir, "bank-consolidation")
    }

    if home, err := os.UserHomeDir(); err == nil && home != "" {
        return filepath.Join(home, ".bank-consolidation")
    }

    if exe, err := os.Executable(); err == nil && exe != "" {
        return filepath.Join(filepath.Dir(exe), "data")
    }

    return "data"
}

func fileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}
