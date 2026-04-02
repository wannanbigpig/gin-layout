package config

import "reflect"

// BuildConfigDiff 生成配置差异摘要。
func BuildConfigDiff(oldConfig, newConfig *Conf) ConfigDiff {
	diff := ConfigDiff{}
	if oldConfig == nil || newConfig == nil {
		return diff
	}

	diff.LoggerChanged = !reflect.DeepEqual(oldConfig.Logger, newConfig.Logger)
	diff.MysqlChanged = !reflect.DeepEqual(oldConfig.Mysql, newConfig.Mysql)
	diff.RedisChanged = !reflect.DeepEqual(oldConfig.Redis, newConfig.Redis)
	diff.JWTChanged = oldConfig.Jwt.TTL != newConfig.Jwt.TTL || oldConfig.Jwt.RefreshTTL != newConfig.Jwt.RefreshTTL
	diff.JWTSecretChanged = oldConfig.Jwt.SecretKey != newConfig.Jwt.SecretKey
	diff.BaseURLChanged = oldConfig.BaseURL != newConfig.BaseURL
	diff.CORSChanged = !reflect.DeepEqual(oldConfig.CorsOrigins, newConfig.CorsOrigins) ||
		!reflect.DeepEqual(oldConfig.CorsMethods, newConfig.CorsMethods) ||
		!reflect.DeepEqual(oldConfig.CorsHeaders, newConfig.CorsHeaders) ||
		!reflect.DeepEqual(oldConfig.CorsExposeHeaders, newConfig.CorsExposeHeaders) ||
		oldConfig.CorsMaxAge != newConfig.CorsMaxAge ||
		oldConfig.CorsCredentials != newConfig.CorsCredentials
	diff.TrustedProxiesChanged = !reflect.DeepEqual(oldConfig.TrustedProxies, newConfig.TrustedProxies)
	diff.LightAppChanged = oldConfig.BasePath != newConfig.BasePath ||
		oldConfig.AppEnv != newConfig.AppEnv ||
		oldConfig.Debug != newConfig.Debug ||
		oldConfig.WatchConfig != newConfig.WatchConfig ||
		oldConfig.Language != newConfig.Language

	if diff.LoggerChanged {
		diff.ChangedFields = append(diff.ChangedFields, "logger.*")
	}
	if diff.MysqlChanged {
		diff.ChangedFields = append(diff.ChangedFields, "mysql.*")
	}
	if diff.RedisChanged {
		diff.ChangedFields = append(diff.ChangedFields, "redis.*")
	}
	if diff.JWTChanged {
		diff.ChangedFields = append(diff.ChangedFields, "jwt.ttl", "jwt.refresh_ttl")
	}
	if diff.JWTSecretChanged {
		diff.ChangedFields = append(diff.ChangedFields, "jwt.secret_key")
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "jwt.secret_key")
	}
	if diff.BaseURLChanged {
		diff.ChangedFields = append(diff.ChangedFields, "app.base_url")
	}
	if diff.CORSChanged {
		diff.ChangedFields = append(diff.ChangedFields, "app.cors_*")
	}
	if diff.TrustedProxiesChanged {
		diff.ChangedFields = append(diff.ChangedFields, "app.trusted_proxies")
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "app.trusted_proxies")
	}
	if oldConfig.Language != newConfig.Language {
		diff.RestartRequiredFields = append(diff.RestartRequiredFields, "app.language")
	}

	return diff
}

// BuildAppliedConfig 返回当前进程应采用的配置快照。
// 对不支持热更新的字段保持旧值，避免配置快照与实际运行状态不一致。
func BuildAppliedConfig(oldConfig, newConfig *Conf, diff ConfigDiff) *Conf {
	if oldConfig == nil {
		return newConfig
	}
	applied := *newConfig
	applied.AppConfig = cloneAppConfig(newConfig.AppConfig)
	applied.Queue = cloneQueueConfig(newConfig.Queue)

	if diff.JWTSecretChanged {
		applied.Jwt.SecretKey = oldConfig.Jwt.SecretKey
	}
	if diff.TrustedProxiesChanged {
		applied.TrustedProxies = cloneStringSlice(oldConfig.TrustedProxies)
	}
	if oldConfig.Language != newConfig.Language {
		applied.Language = oldConfig.Language
	}

	return &applied
}
