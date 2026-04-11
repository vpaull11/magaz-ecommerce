package middleware

import "net/http"

// SecurityHeaders adds OWASP-recommended security headers to every response.
// Protects against XSS, clickjacking, MIME sniffing, and information leakage.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Prevent MIME-type sniffing attacks
		h.Set("X-Content-Type-Options", "nosniff")

		// Deny embedding in iframes — blocks clickjacking
		h.Set("X-Frame-Options", "DENY")

		// Don't leak full URL in Referer header when following external links
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Disable browser features we don't use
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

		// Content Security Policy — whitelist of allowed resource origins.
		// style-src includes googleapis.com for Google Fonts CSS.
		// font-src includes gstatic.com for font files.
		// img-src includes data: for base64 images.
		// frame-ancestors 'none' — second layer of clickjacking protection.
		// form-action 'self' — forms may only submit to same origin.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: blob:; "+
				"script-src 'self' 'unsafe-inline'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self';",
		)

		// X-XSS-Protection: legacy but still used by older browsers
		h.Set("X-XSS-Protection", "1; mode=block")

		// NOTE: Strict-Transport-Security (HSTS) is intentionally omitted here.
		// It must only be set when the server is serving over HTTPS.
		// Enable HSTS in your reverse proxy (nginx/caddy) or add:
		//   h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		// only after confirming HTTPS is fully working.

		next.ServeHTTP(w, r)
	})
}
