package webapp

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Статика должна попасть в бинарник: без неё Mini App открывался бы пустой страницей
// только в проде, где это уже некому заметить.
func TestStatic_ServesEmbeddedIndex(t *testing.T) {
	env := newTestEnv(t)

	resp := env.doWithAuth(http.MethodGet, "/", nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	html := string(body)
	assert.Contains(t, html, "telegram-web-app.js", "Telegram SDK must be loaded")
	assert.Contains(t, html, "/app.js")
	assert.Contains(t, html, "/style.css")
	assert.Contains(t, html, `id="chat-context-title"`)
}

func TestStatic_ServesScriptAndStyles(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/app.js", "javascript", "tgWebAppStartParam"},
		{"/style.css", "css", "--tg-theme-bg-color"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp := env.doWithAuth(http.MethodGet, tt.path, nil, "")
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, resp.Header.Get("Content-Type"), tt.contentType)
			assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), tt.contains)
		})
	}
}

// Mini App — одностраничное приложение: перезагрузка на внутреннем маршруте
// должна отдавать разметку, а не 404.
func TestStatic_FallsBackToIndexForAppRoutes(t *testing.T) {
	env := newTestEnv(t)

	resp := env.doWithAuth(http.MethodGet, "/settings", nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "<title>"), "expected the app shell")
}

// Статика не должна выпускать наружу файлы за пределами каталога web.
func TestStatic_DoesNotEscapeAssetRoot(t *testing.T) {
	env := newTestEnv(t)

	for _, path := range []string{"/../server.go", "/..%2fserver.go", "/web/index.html"} {
		t.Run(path, func(t *testing.T) {
			resp := env.doWithAuth(http.MethodGet, path, nil, "")
			body, _ := io.ReadAll(resp.Body)
			assert.NotContains(t, string(body), "package webapp", "must not serve Go sources")
		})
	}
}
