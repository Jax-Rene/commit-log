package locale

// Pick returns the text matching the request language, defaulting to Chinese.
func Pick(language, english, chinese string) string {
	if NormalizeLanguage(language) == LanguageEnglish {
		if english != "" {
			return english
		}
		return chinese
	}
	if chinese != "" {
		return chinese
	}
	return english
}
