package sensitive

import "strings"

func maskString(s string) string {
	if s == "" {
		return s
	}
	return applyMatchers(s, maskBankCard, maskIdCard, maskPhone, maskEmail)
}

func maskValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		if val == "" {
			return val
		}
		return maskSensitiveString(val)
	case map[string]interface{}:
		return maskMap(val, getCommonFields())
	case []interface{}:
		return maskArrayWithFields(val, getCommonFields())
	default:
		return v
	}
}

func maskSensitiveString(s string) string {
	if s == "" {
		return s
	}

	if prefix, ok := authPrefix(s); ok {
		return maskAuthToken(s, prefix)
	}

	masked := applyMatchers(s, maskBankCard, maskIdCard, maskPhone, maskEmail)
	if masked != s {
		return masked
	}
	return maskDefault(s)
}

func authPrefix(s string) (string, bool) {
	switch sLower := strings.ToLower(s); {
	case strings.HasPrefix(s, "eyJ"):
		return "", false
	case strings.HasPrefix(sLower, "bearer "):
		return "bearer ", true
	case strings.HasPrefix(sLower, "basic "):
		return "basic ", true
	default:
		return "", false
	}
}

func applyMatchers(s string, replacers ...func(string) string) string {
	result := s
	if bankCardRegex.MatchString(result) {
		result = bankCardRegex.ReplaceAllStringFunc(result, replacers[0])
	}
	if idCardRegex.MatchString(result) {
		result = idCardRegex.ReplaceAllStringFunc(result, replacers[1])
	}
	if phoneRegex.MatchString(result) {
		result = phoneRegex.ReplaceAllStringFunc(result, replacers[2])
	}
	if emailRegex.MatchString(result) {
		result = emailRegex.ReplaceAllStringFunc(result, replacers[3])
	}
	return result
}

func maskAuthToken(s, prefix string) string {
	if strings.HasPrefix(strings.ToLower(s), prefix) {
		tokenPart := s[len(prefix):]
		return prefix + maskToken(tokenPart)
	}
	parts := strings.SplitN(s, " ", 2)
	if len(parts) == 2 {
		return parts[0] + " " + maskToken(parts[1])
	}
	return maskDefault(s)
}

func isSensitiveField(fieldName string, sensitiveFields map[string]bool) bool {
	if sensitiveFields[fieldName] {
		return true
	}
	if len(fieldName) < 3 {
		return false
	}
	for keyword := range sensitiveFields {
		if len(keyword) <= len(fieldName) && strings.Contains(fieldName, keyword) {
			return true
		}
	}
	return false
}

func maskToken(token string) string {
	length := len(token)
	if length <= maskTokenPrefixLen+maskTokenSuffixLen {
		return strings.Repeat("*", length)
	}
	return token[:maskTokenPrefixLen] + "***" + token[length-maskTokenSuffixLen:]
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return maskDefault(phone)
	}
	return phone[:maskPhonePrefixLen] + "****" + phone[11-maskPhoneSuffixLen:]
}

func maskEmail(email string) string {
	idx := strings.IndexByte(email, '@')
	if idx == -1 || idx == 0 {
		return maskDefault(email)
	}

	localPart := email[:idx]
	domain := email[idx:]
	if len(localPart) <= maskEmailPrefixLen {
		return strings.Repeat("*", len(localPart)) + domain
	}
	return localPart[:maskEmailPrefixLen] + "***" + domain
}

func maskIdCard(idCard string) string {
	switch len(idCard) {
	case 15:
		return idCard[:maskIdCardPrefixLen] + "******" + idCard[15-maskIdCardSuffixLen:]
	case 18:
		return idCard[:maskIdCardPrefixLen] + "********" + idCard[18-maskIdCardSuffixLen:]
	default:
		return maskDefault(idCard)
	}
}

func maskBankCard(cardNo string) string {
	length := len(cardNo)
	if length < maskBankCardPrefixLen+maskBankCardSuffixLen {
		return maskDefault(cardNo)
	}
	maskLen := length - maskBankCardPrefixLen - maskBankCardSuffixLen
	return cardNo[:maskBankCardPrefixLen] + strings.Repeat("*", maskLen) + cardNo[length-maskBankCardSuffixLen:]
}

func maskDefault(s string) string {
	length := len(s)
	if length <= maskDefaultPrefixLen+maskDefaultSuffixLen {
		return strings.Repeat("*", length)
	}
	maskLen := length - maskDefaultPrefixLen - maskDefaultSuffixLen
	return s[:maskDefaultPrefixLen] + strings.Repeat("*", maskLen) + s[length-maskDefaultSuffixLen:]
}
