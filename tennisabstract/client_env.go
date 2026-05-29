package tennisabstract

import (
	"os"
	"strings"
)

// CacheConfiguredFromEnv reports whether REDIS_URL or REDIS_ADDR is set.
func CacheConfiguredFromEnv() bool {
	return strings.TrimSpace(os.Getenv("REDIS_URL")) != "" ||
		strings.TrimSpace(os.Getenv("REDIS_ADDR")) != ""
}

// ClientOptionsFromEnv returns HTTP rate-limit options and, when Redis env is configured,
// WithCache and WithCacheTTL.
func ClientOptionsFromEnv() ([]Option, error) {
	opts := HTTPClientOptionsFromEnv()
	if !CacheConfiguredFromEnv() {
		return opts, nil
	}
	cache, err := NewRedisCacheFromEnv()
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		WithCache(cache),
		WithCacheTTL(CacheTTLFromEnv()),
	)
	return opts, nil
}

// CareerClientOptionsFromEnv returns HTTP rate-limit options and WithCareerCacheDir
// from TENNISABSTRACT_CAREER_DIR (or default).
func CareerClientOptionsFromEnv() []Option {
	opts := HTTPClientOptionsFromEnv()
	opts = append(opts, WithCareerCacheDir(CareerCacheDirFromEnv()))
	return opts
}
