package config

import "github.com/spf13/viper"

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.listen", ":8080")
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "120s")

	v.SetDefault("dns.query_timeout", "2s")
	v.SetDefault("dns.per_resolver_concurrency", 100)
	v.SetDefault("dns.default_protocol", "udp")
	v.SetDefault("dns.retry", 1)

	v.SetDefault("resolvers.builtin", true)

	v.SetDefault("storage.driver", "sqlite")
	v.SetDefault("storage.dsn", "file:./dnsmon.db?cache=shared&_fk=1")
	v.SetDefault("storage.retention_days", 30)

	v.SetDefault("cache.driver", "none")
	v.SetDefault("cache.ttl", "60s")
	v.SetDefault("cache.size", 10000)

	v.SetDefault("ratelimit.enabled", true)
	v.SetDefault("ratelimit.anon_per_min", 30)
	v.SetDefault("ratelimit.key_per_min", 600)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")
}
