package config

// CloneConf 返回配置的深拷贝，避免测试场景下共享可变引用。
func CloneConf(src *Conf) *Conf {
	if src == nil {
		return &Conf{}
	}

	cloned := *src
	cloned.AppConfig = cloneAppConfig(src.AppConfig)
	cloned.Queue = cloneQueueConfig(src.Queue)
	return &cloned
}

// ReplaceConfigForTesting 替换当前配置并返回恢复函数。
func ReplaceConfigForTesting(cfg *Conf) func() {
	previous := CloneConf(GetConfig())

	if cfg == nil {
		setActiveConfig(&Conf{})
	} else {
		setActiveConfig(CloneConf(cfg))
	}

	return func() {
		setActiveConfig(previous)
	}
}

// UpdateConfigForTesting 在当前配置副本上应用变更并返回恢复函数。
func UpdateConfigForTesting(mutator func(cfg *Conf)) func() {
	next := CloneConf(GetConfig())
	if mutator != nil {
		mutator(next)
	}
	return ReplaceConfigForTesting(next)
}
