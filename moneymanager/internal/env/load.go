package env

import "github.com/joho/godotenv"

// Load reads moneymanager/.env then polymarket/.env (when run from moneymanager/).
// Later files do not override variables already set.
func Load() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../polymarket/.env")
}
