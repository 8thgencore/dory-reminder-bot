package webapp

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
)

//go:embed web
var embeddedAssets embed.FS

// staticHandler отдаёт файлы Mini App.
//
// В prod файлы берутся из бинарника, в dev — с диска, чтобы правка разметки
// подхватывалась без пересборки (air перезапускает процесс только на изменение .go).
func (s *server) staticHandler() http.Handler {
	assets := s.assetFS()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Telegram WebView может долго держать app.js по прежнему URL. Статика
		// небольшая, поэтому запрещаем её хранение и всегда отдаём текущий билд.
		w.Header().Set("Cache-Control", "no-store")

		// Mini App — одностраничное приложение: неизвестные пути отдают index.html,
		// чтобы перезагрузка на внутреннем маршруте не давала 404.
		path := filepath.Clean(r.URL.Path)
		if path == "/" || filepath.Ext(path) == "" {
			serveIndex(w, r, assets)
			return
		}

		http.FileServer(http.FS(assets)).ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		http.Error(w, "Mini App assets are missing", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Разметка ссылается на статику по именам без хэшей, поэтому кэшировать её нельзя.
	w.Header().Set("Cache-Control", "no-store")
	// Встроенные через go:embed файлы не имеют времени модификации, поэтому берём
	// время старта процесса: для одного билда оно постоянно.
	http.ServeContent(w, r, "index.html", startedAt, bytes.NewReader(index))
}

// startedAt служит временем модификации статики.
var startedAt = time.Now()

// assetFS выбирает источник статики в зависимости от окружения.
func (s *server) assetFS() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "web")
	if err != nil {
		// Каталог гарантирован директивой go:embed — сюда попасть нельзя.
		panic("webapp: embedded assets are broken: " + err.Error())
	}

	if s.env != config.Dev {
		return sub
	}

	dir, ok := sourceAssetDir()
	if !ok {
		s.log.Warn("Falling back to embedded assets: source directory not found")
		return sub
	}
	s.log.Info("Serving Mini App assets from disk", "dir", dir)

	return os.DirFS(dir)
}

// sourceAssetDir находит каталог web рядом с исходником этого файла.
func sourceAssetDir() (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}

	dir := filepath.Join(filepath.Dir(file), "web")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", false
	}

	return dir, true
}
