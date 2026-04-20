package config

// GetConfigFrom 通过指定 provider 获取配置，并保证返回值非 nil。
func GetConfigFrom(provider func() *Conf) *Conf {
	if provider == nil {
		return &Conf{}
	}

	cfg := provider()
	if cfg == nil {
		return &Conf{}
	}
	return cfg
}
