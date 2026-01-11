package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/commitlog/internal/locale"
	"github.com/gin-gonic/gin"
)

const (
	localeContextKey     = "__request_locale"
	languageCookieName   = "cl_lang"
	languageCookieMaxAge = 365 * 24 * 60 * 60
)

var countryHeaderCandidates = []string{
	"CF-IPCountry",
	"X-Geo-Country",
	"X-Forwarded-Country",
	"X-Country-Code",
}

// LocaleMiddleware resolves request language and sets headers for downstream caching.
func (a *API) LocaleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		pref := a.requestLocale(c)
		if pref.HTMLLang != "" {
			c.Header("Content-Language", pref.HTMLLang)
		}
		varyHeaders := append([]string{"Accept-Language"}, countryHeaderCandidates...)
		if readLanguageCookie(c) != "" || locale.NormalizeLanguage(c.Query("lang")) != "" {
			varyHeaders = append(varyHeaders, "Cookie")
		}
		appendVaryHeader(c, varyHeaders...)
		c.Next()
	}
}

func (a *API) requestLocale(c *gin.Context) locale.Preference {
	if cached, exists := c.Get(localeContextKey); exists {
		if pref, ok := cached.(locale.Preference); ok {
			return pref
		}
	}
	language, persist := a.resolveLanguage(c)
	pref := locale.PreferenceForLanguage(language)
	if persist {
		a.persistLanguage(c, pref.Language)
	}
	c.Set(localeContextKey, pref)
	return pref
}

func (a *API) resolveLanguage(c *gin.Context) (string, bool) {
	if override := locale.NormalizeLanguage(c.Query("lang")); override != "" {
		return override, true
	}
	if cookie := readLanguageCookie(c); cookie != "" {
		return cookie, false
	}
	if preferred, ok := a.preferredLanguage(c); ok {
		return preferred, true
	}
	country := readCountryHeader(c)
	if country != "" {
		return locale.LanguageFromCountryCode(country), false
	}
	if fromHeader := locale.LanguageFromAcceptLanguage(c.GetHeader("Accept-Language")); fromHeader != "" {
		return fromHeader, false
	}
	return locale.LanguageChinese, false
}

func readLanguageCookie(c *gin.Context) string {
	value, err := c.Cookie(languageCookieName)
	if err != nil {
		return ""
	}
	return locale.NormalizeLanguage(value)
}

func (a *API) persistLanguage(c *gin.Context, language string) {
	normalized := locale.NormalizeLanguage(language)
	if normalized == "" {
		return
	}
	secure := strings.EqualFold(a.detectScheme(c), "https")
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     languageCookieName,
		Value:    normalized,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		MaxAge:   languageCookieMaxAge,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) preferredLanguage(c *gin.Context) (string, bool) {
	if a == nil || a.system == nil {
		return "", false
	}
	language, ok, err := a.system.PreferredLanguage()
	if err != nil {
		c.Error(err)
		return "", false
	}
	return language, ok
}

func buildLanguageSwitch(c *gin.Context) map[string]string {
	path := "/"
	rawQuery := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
		rawQuery = c.Request.URL.RawQuery
	}
	values, _ := url.ParseQuery(rawQuery)
	values.Set("lang", locale.LanguageChinese)
	zhURL := path
	if encoded := values.Encode(); encoded != "" {
		zhURL += "?" + encoded
	}
	values.Set("lang", locale.LanguageEnglish)
	enURL := path
	if encoded := values.Encode(); encoded != "" {
		enURL += "?" + encoded
	}
	return map[string]string{
		"zh": zhURL,
		"en": enURL,
	}
}

func readCountryHeader(c *gin.Context) string {
	for _, header := range countryHeaderCandidates {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		parts := strings.Split(value, ",")
		if len(parts) == 0 {
			continue
		}
		candidate := strings.TrimSpace(parts[0])
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func appendVaryHeader(c *gin.Context, headers ...string) {
	existing := c.Writer.Header().Get("Vary")
	seen := make(map[string]struct{})
	order := make([]string, 0, len(headers))
	for _, token := range strings.Split(existing, ",") {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		order = append(order, trimmed)
	}
	for _, header := range headers {
		trimmed := strings.TrimSpace(header)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		order = append(order, trimmed)
	}
	if len(order) > 0 {
		c.Header("Vary", strings.Join(order, ", "))
	}
}
