package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	fmt.Println("🚀 Запуск сервера обработки изображений...")
	fmt.Println("📍 Адрес: http://localhost:8080")

	// Создаем необходимые папки
	os.MkdirAll("uploads", 0755)
	os.MkdirAll("static", 0755)

	// Проверяем статические файлы
	if _, err := os.Stat("static/index.html"); os.IsNotExist(err) {
		createStaticFiles()
	}

	// Настройка маршрутов
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/process", handleProcess)
	http.HandleFunc("/api/filters", handleFilters)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Запуск сервера
	fmt.Println("✅ Сервер готов к работе!")
	fmt.Println("📌 Функции:")
	fmt.Println("  • Загрузка изображений")
	fmt.Println("  • 6 фильтров")
	fmt.Println("  • Поворот и отражение")
	fmt.Println("  • Изменение размера")
	fmt.Println("  • Скачивание результата")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("❌ Ошибка: %v\n", err)
	}
}

// serveHome - главная страница
func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "static/index.html")
		return
	}
	http.NotFound(w, r)
}

// handleUpload - загрузка файла
func handleUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		sendJSONError(w, "Только POST метод", http.StatusMethodNotAllowed)
		return
	}

	// Максимальный размер 20MB
	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		sendJSONError(w, "Файл слишком большой (макс 20MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		sendJSONError(w, "Файл не найден", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Сохраняем файл
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), sanitizeFilename(header.Filename))
	filepath := "uploads/" + filename

	dst, err := os.Create(filepath)
	if err != nil {
		sendJSONError(w, "Ошибка сохранения", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		sendJSONError(w, "Ошибка копирования", http.StatusInternalServerError)
		return
	}

	fmt.Printf("[UPLOAD] %s (%.2f MB)\n", header.Filename, float64(header.Size)/1024/1024)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Файл успешно загружен",
		"filename": filename,
		"size":     header.Size,
		"url":      "/uploads/" + filename,
	})
}

// handleProcess - обработка изображения
func handleProcess(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if r.Method != "POST" {
		http.Error(w, "Только POST метод", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		http.Error(w, "Файл слишком большой", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Файл не найден", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Читаем данные
	imgData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Ошибка чтения", http.StatusInternalServerError)
		return
	}

	// Получаем параметры
	width, _ := strconv.Atoi(r.FormValue("width"))
	height, _ := strconv.Atoi(r.FormValue("height"))

	quality, err := strconv.Atoi(r.FormValue("quality"))
	if err != nil || quality <= 0 || quality > 100 {
		quality = 85
	}

	format := r.FormValue("format")
	if format == "" {
		format = "jpg"
	}

	filter := r.FormValue("filter")
	rotate, _ := strconv.ParseFloat(r.FormValue("rotate"), 64)
	flip := r.FormValue("flip")

	// Декодируем изображение
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		http.Error(w, "Неверный формат изображения", http.StatusBadRequest)
		return
	}

	// Применяем операции
	if rotate != 0 {
		img = rotateImage(img, rotate)
	}

	if flip != "" && flip != "none" {
		img = flipImage(img, flip)
	}

	if filter != "" && filter != "none" {
		img = applyFilter(img, filter)
	}

	if width > 0 || height > 0 {
		img = resizeImage(img, width, height)
	}

	// Кодируем результат
	result, err := encodeImage(img, format, quality)
	if err != nil {
		http.Error(w, "Ошибка кодирования", http.StatusInternalServerError)
		return
	}

	// Отправляем результат
	w.Header().Set("Content-Type", getContentType(format))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"processed_%s\"", header.Filename))
	w.Write(result)

	elapsed := time.Since(startTime)
	fmt.Printf("[PROCESS] %s -> %s (%s) за %v\n", header.Filename, format, filter, elapsed)
}

// handleFilters - список фильтров
func handleFilters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	filters := []map[string]string{
		{"id": "none", "name": "Без фильтра", "icon": "🔄"},
		{"id": "grayscale", "name": "Черно-белый", "icon": "⚫"},
		{"id": "sepia", "name": "Сепия", "icon": "🟤"},
		{"id": "invert", "name": "Инверсия", "icon": "🔄"},
		{"id": "cool", "name": "Холодный", "icon": "❄️"},
		{"id": "warm", "name": "Теплый", "icon": "🔥"},
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"filters": filters,
	})
}

// Функции обработки изображений
func rotateImage(img image.Image, angle float64) image.Image {
	if angle == 0 {
		return img
	}

	rad := angle * math.Pi / 180
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	sin, cos := math.Sin(rad), math.Cos(rad)
	newW := int(math.Ceil(math.Abs(float64(w)*cos) + math.Abs(float64(h)*sin)))
	newH := int(math.Ceil(math.Abs(float64(w)*sin) + math.Abs(float64(h)*cos)))

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	cx, cy := float64(w)/2, float64(h)/2
	newCx, newCy := float64(newW)/2, float64(newH)/2

	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := (float64(x)-newCx)*cos + (float64(y)-newCy)*sin + cx
			srcY := -(float64(x)-newCx)*sin + (float64(y)-newCy)*cos + cy

			if srcX >= 0 && srcX < float64(w) && srcY >= 0 && srcY < float64(h) {
				dst.Set(x, y, img.At(int(srcX), int(srcY)))
			}
		}
	}

	return dst
}

func flipImage(img image.Image, direction string) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	w, h := bounds.Dx(), bounds.Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var srcX, srcY int

			switch direction {
			case "horizontal":
				srcX = w - x - 1
				srcY = y
			case "vertical":
				srcX = x
				srcY = h - y - 1
			case "both":
				srcX = w - x - 1
				srcY = h - y - 1
			default:
				srcX, srcY = x, y
			}

			dst.Set(x, y, img.At(srcX, srcY))
		}
	}

	return dst
}

func resizeImage(img image.Image, width, height int) image.Image {
	if width <= 0 && height <= 0 {
		return img
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if width <= 0 {
		ratio := float64(height) / float64(h)
		width = int(float64(w) * ratio)
	} else if height <= 0 {
		ratio := float64(width) / float64(w)
		height = int(float64(h) * ratio)
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	xRatio := float64(w) / float64(width)
	yRatio := float64(h) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := int(float64(x) * xRatio)
			srcY := int(float64(y) * yRatio)

			if srcX < w && srcY < h {
				dst.Set(x, y, img.At(srcX, srcY))
			}
		}
	}

	return dst
}

func applyFilter(img image.Image, filter string) image.Image {
	switch filter {
	case "grayscale":
		return applyGrayscale(img)
	case "sepia":
		return applySepia(img)
	case "invert":
		return applyInvert(img)
	case "cool":
		return applyCool(img)
	case "warm":
		return applyWarm(img)
	default:
		return img
	}
}

func applyGrayscale(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			gray := uint32(0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8))
			gray8 := uint8(gray)
			dst.Set(x, y, color.RGBA{gray8, gray8, gray8, uint8(a >> 8)})
		}
	}
	return dst
}

func applySepia(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			tr := float64(r>>8)*0.393 + float64(g>>8)*0.769 + float64(b>>8)*0.189
			tg := float64(r>>8)*0.349 + float64(g>>8)*0.686 + float64(b>>8)*0.168
			tb := float64(r>>8)*0.272 + float64(g>>8)*0.534 + float64(b>>8)*0.131

			tr = math.Min(255, tr)
			tg = math.Min(255, tg)
			tb = math.Min(255, tb)

			dst.Set(x, y, color.RGBA{
				uint8(tr), uint8(tg), uint8(tb), uint8(a >> 8),
			})
		}
	}
	return dst
}

func applyInvert(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				255 - uint8(r>>8),
				255 - uint8(g>>8),
				255 - uint8(b>>8),
				uint8(a >> 8),
			})
		}
	}
	return dst
}

func applyCool(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			tr := float64(r>>8) * 0.9
			tg := float64(g>>8) * 0.9
			tb := float64(b>>8) * 1.1

			tr = math.Min(255, tr)
			tg = math.Min(255, tg)
			tb = math.Min(255, tb)

			dst.Set(x, y, color.RGBA{
				uint8(tr), uint8(tg), uint8(tb), uint8(a >> 8),
			})
		}
	}
	return dst
}

func applyWarm(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			tr := float64(r>>8) * 1.1
			tg := float64(g>>8) * 1.0
			tb := float64(b>>8) * 0.9

			tr = math.Min(255, tr)
			tg = math.Min(255, tg)
			tb = math.Min(255, tb)

			dst.Set(x, y, color.RGBA{
				uint8(tr), uint8(tg), uint8(tb), uint8(a >> 8),
			})
		}
	}
	return dst
}

// Вспомогательные функции
func encodeImage(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		return buf.Bytes(), err
	case "png":
		err := png.Encode(&buf, img)
		return buf.Bytes(), err
	default:
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		return buf.Bytes(), err
	}
}

func getContentType(format string) string {
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func sanitizeFilename(filename string) string {
	unsafe := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, s := range unsafe {
		filename = strings.ReplaceAll(filename, s, "_")
	}
	return filename
}

func sendJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"success": false,
	})
}

// Создание статических файлов
func createStaticFiles() {
	fmt.Println("📄 Создаю статические файлы...")

	// Создаем index.html
	createHTMLFile()

	// Создаем style.css
	createCSSFile()

	// Создаем script.js
	createJSFile()

	fmt.Println("✅ Статические файлы созданы")
}

func createHTMLFile() {
	html := `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🎨 Обработчик изображений</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>🎨 Обработчик изображений</h1>
            <p class="subtitle">Загрузите изображение для применения фильтров и редактирования</p>
        </header>
        
        <!-- Загрузка -->
        <section class="upload-section">
            <div class="upload-area" id="uploadArea">
                <input type="file" id="fileInput" accept="image/*">
                <div class="upload-icon">📁</div>
                <h2>Перетащите изображение сюда</h2>
                <p>или нажмите для выбора файла</p>
                <p class="file-info">Поддерживаются: JPG, PNG, GIF, BMP, WebP (до 20MB)</p>
            </div>
        </section>
        
        <!-- Превью -->
        <section class="preview-container" id="previewContainer">
            <h2>Предпросмотр</h2>
            <div class="image-grid">
                <div class="image-box">
                    <h3>Оригинал</h3>
                    <img id="originalImg" alt="Оригинал">
                    <div class="image-info" id="originalInfo"></div>
                </div>
                <div class="image-box">
                    <h3>Результат</h3>
                    <img id="resultImg" alt="Результат">
                    <div class="image-info" id="resultInfo"></div>
                </div>
            </div>
        </section>
        
        <!-- Управление -->
        <section class="controls-section" id="controlsSection">
            <h2>Настройки обработки</h2>
            
            <div class="controls-grid">
                <!-- Фильтры -->
                <div class="control-group">
                    <h3>🎨 Фильтры</h3>
                    <div class="filters" id="filtersContainer">
                        <!-- Фильтры загрузятся через JS -->
                    </div>
                </div>
                
                <!-- Операции -->
                <div class="control-group">
                    <h3>🔄 Операции</h3>
                    <div class="operation">
                        <label>Поворот: <span id="rotateValue">0°</span></label>
                        <input type="range" id="rotateSlider" min="-180" max="180" value="0" class="slider">
                        <div class="quick-buttons">
                            <button class="small-btn" data-rotate="-90">↺ -90°</button>
                            <button class="small-btn" data-rotate="90">↻ +90°</button>
                            <button class="small-btn" data-rotate="180">🔄 180°</button>
                        </div>
                    </div>
                    
                    <div class="operation">
                        <label>Отражение:</label>
                        <div class="flip-buttons">
                            <button class="small-btn active" data-flip="none">Нет</button>
                            <button class="small-btn" data-flip="horizontal">↔ Гориз.</button>
                            <button class="small-btn" data-flip="vertical">↕ Вертик.</button>
                        </div>
                    </div>
                </div>
                
                <!-- Размер -->
                <div class="control-group">
                    <h3>📏 Размер</h3>
                    <div class="size-controls">
                        <div class="size-input">
                            <label>Ширина:</label>
                            <input type="number" id="widthInput" min="10" max="4000" value="800">
                            <span>px</span>
                        </div>
                        <div class="size-input">
                            <label>Высота:</label>
                            <input type="number" id="heightInput" min="10" max="4000" value="600">
                            <span>px</span>
                        </div>
                    </div>
                    <div class="checkbox">
                        <input type="checkbox" id="keepAspect" checked>
                        <label for="keepAspect">Сохранять пропорции</label>
                    </div>
                </div>
                
                <!-- Настройки -->
                <div class="control-group">
                    <h3>⚙️ Настройки</h3>
                    <div class="settings">
                        <div class="setting">
                            <label>Формат:</label>
                            <select id="formatSelect">
                                <option value="jpg">JPEG</option>
                                <option value="png">PNG</option>
                            </select>
                        </div>
                        <div class="setting">
                            <label>Качество: <span id="qualityValue">85%</span></label>
                            <input type="range" id="qualitySlider" min="1" max="100" value="85" class="slider">
                        </div>
                    </div>
                </div>
            </div>
        </section>
        
        <!-- Загрузка -->
        <div class="loading" id="loading">
            <div class="spinner"></div>
            <p>Обрабатываю изображение...</p>
        </div>
        
        <!-- Результат -->
        <div class="result-container" id="resultContainer">
            <div class="result-card">
                <h3>✅ Обработка завершена!</h3>
                <p>Изображение успешно обработано и готово к скачиванию</p>
            </div>
        </div>
        
        <!-- Кнопки действий -->
        <div class="action-buttons">
            <button class="btn primary-btn" id="processBtn" disabled>
                ⚙️ Обработать изображение
            </button>
            <button class="btn secondary-btn" id="downloadBtn" disabled>
                💾 Скачать результат
            </button>
            <button class="btn danger-btn" id="resetBtn">
                🔄 Сбросить всё
            </button>
        </div>
    </div>

    <script src="/static/script.js"></script>
</body>
</html>`

	os.WriteFile("static/index.html", []byte(html), 0644)
}

func createCSSFile() {
	css := `/* Основные стили */
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
    font-family: 'Segoe UI', Arial, sans-serif;
}

body {
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    min-height: 100vh;
    padding: 20px;
    display: flex;
    justify-content: center;
    align-items: center;
}

.container {
    background: white;
    border-radius: 20px;
    padding: 40px;
    box-shadow: 0 20px 60px rgba(0,0,0,0.3);
    max-width: 1200px;
    width: 100%;
    margin: 20px;
}

/* Заголовок */
header {
    text-align: center;
    margin-bottom: 40px;
}

h1 {
    color: #333;
    font-size: 2.5em;
    margin-bottom: 10px;
}

.subtitle {
    color: #666;
    font-size: 1.2em;
}

/* Область загрузки */
.upload-section {
    margin: 40px 0;
}

.upload-area {
    border: 3px dashed #667eea;
    border-radius: 15px;
    padding: 60px 20px;
    text-align: center;
    cursor: pointer;
    transition: all 0.3s;
}

.upload-area:hover {
    background: #f8f9ff;
    border-color: #764ba2;
}

.upload-area.dragover {
    background: #667eea20;
    border-color: #4CAF50;
}

#fileInput {
    display: none;
}

.upload-icon {
    font-size: 64px;
    color: #667eea;
    margin-bottom: 20px;
}

.file-info {
    color: #666;
    font-size: 14px;
    margin-top: 10px;
}

/* Предпросмотр */
.preview-container {
    display: none;
    margin: 40px 0;
}

.image-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 30px;
    margin-top: 20px;
}

.image-box {
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 10px 30px rgba(0,0,0,0.1);
}

.image-box h3 {
    background: #667eea;
    color: white;
    padding: 15px;
    margin: 0;
    text-align: center;
    font-size: 1.2em;
}

.image-box img {
    width: 100%;
    height: 300px;
    object-fit: contain;
    background: #f5f5f5;
    display: block;
}

.image-info {
    padding: 10px;
    background: #f8f9fa;
    text-align: center;
    font-size: 0.9em;
    color: #666;
}

/* Управление */
.controls-section {
    display: none;
    margin: 40px 0;
}

.controls-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 30px;
    margin-top: 20px;
}

.control-group {
    background: #f8f9fa;
    padding: 25px;
    border-radius: 15px;
}

.control-group h3 {
    color: #333;
    margin-bottom: 20px;
    font-size: 1.3em;
}

/* Фильтры */
.filters {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
}

.filter-btn {
    padding: 12px;
    border: 2px solid #ddd;
    background: white;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.3s;
    font-size: 0.9em;
    text-align: center;
}

.filter-btn:hover {
    border-color: #667eea;
    background: #f0f2ff;
}

.filter-btn.active {
    background: #667eea;
    color: white;
    border-color: #667eea;
}

/* Операции */
.operation {
    margin-bottom: 20px;
}

.operation label {
    display: block;
    margin-bottom: 10px;
    font-weight: 600;
    color: #444;
}

.slider {
    width: 100%;
    margin: 10px 0;
    height: 6px;
    border-radius: 3px;
    background: #ddd;
    outline: none;
}

.quick-buttons, .flip-buttons {
    display: flex;
    gap: 10px;
    margin-top: 10px;
}

.small-btn {
    padding: 8px 15px;
    border: 2px solid #ddd;
    background: white;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9em;
    flex: 1;
}

.small-btn:hover {
    border-color: #667eea;
}

.small-btn.active {
    background: #667eea;
    color: white;
    border-color: #667eea;
}

/* Размер */
.size-controls {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 15px;
    margin-bottom: 15px;
}

.size-input {
    display: flex;
    flex-direction: column;
    gap: 5px;
}

.size-input input {
    padding: 10px;
    border: 2px solid #ddd;
    border-radius: 8px;
    font-size: 1em;
}

.size-input span {
    font-size: 0.9em;
    color: #666;
}

.checkbox {
    display: flex;
    align-items: center;
    gap: 10px;
}

.checkbox input {
    width: 18px;
    height: 18px;
}

/* Настройки */
.settings {
    display: flex;
    flex-direction: column;
    gap: 20px;
}

.setting {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.setting select {
    padding: 10px;
    border: 2px solid #ddd;
    border-radius: 8px;
    font-size: 1em;
}

/* Кнопки */
.action-buttons {
    display: flex;
    gap: 20px;
    justify-content: center;
    margin: 40px 0;
    flex-wrap: wrap;
}

.btn {
    padding: 16px 32px;
    border: none;
    border-radius: 10px;
    font-size: 1em;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.3s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    min-width: 200px;
}

.btn:disabled {
    background: #ccc;
    cursor: not-allowed;
    opacity: 0.6;
}

.primary-btn {
    background: linear-gradient(135deg, #667eea, #764ba2);
    color: white;
}

.primary-btn:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 10px 20px rgba(102, 126, 234, 0.4);
}

.secondary-btn {
    background: #4CAF50;
    color: white;
}

.secondary-btn:hover:not(:disabled) {
    background: #45a049;
    transform: translateY(-2px);
}

.danger-btn {
    background: #f44336;
    color: white;
}

.danger-btn:hover {
    background: #d32f2f;
    transform: translateY(-2px);
}

/* Загрузка */
.loading {
    display: none;
    text-align: center;
    margin: 30px 0;
}

.spinner {
    border: 5px solid #f3f3f3;
    border-top: 5px solid #667eea;
    border-radius: 50%;
    width: 60px;
    height: 60px;
    animation: spin 1s linear infinite;
    margin: 0 auto 20px;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

/* Результат */
.result-container {
    display: none;
    margin-top: 40px;
}

.result-card {
    background: #e8f5e9;
    padding: 30px;
    border-radius: 15px;
    border-left: 5px solid #4CAF50;
    text-align: center;
}

.result-card h3 {
    color: #2e7d32;
    margin-bottom: 15px;
    font-size: 1.5em;
}

/* Адаптивность */
@media (max-width: 768px) {
    .container {
        padding: 20px;
    }
    
    .action-buttons {
        flex-direction: column;
    }
    
    .btn {
        width: 100%;
    }
    
    .controls-grid {
        grid-template-columns: 1fr;
    }
    
    .upload-area {
        padding: 40px 20px;
    }
}`

	os.WriteFile("static/style.css", []byte(css), 0644)
}

func createJSFile() {
	js := `// Конфигурация
const CONFIG = {
    serverUrl: '',
    maxFileSize: 20 * 1024 * 1024, // 20MB
    allowedTypes: ['image/jpeg', 'image/png', 'image/gif', 'image/bmp', 'image/webp']
};

// Состояние приложения
let state = {
    originalImage: null,
    processedImage: null,
    originalFile: null,
    settings: {
        filter: 'none',
        rotate: 0,
        flip: 'none',
        width: 800,
        height: 600,
        format: 'jpg',
        quality: 85
    }
};

// Инициализация
document.addEventListener('DOMContentLoaded', () => {
    initUpload();
    initFilters();
    initControls();
    initActions();
});

// Загрузка файлов
function initUpload() {
    const uploadArea = document.getElementById('uploadArea');
    const fileInput = document.getElementById('fileInput');
    const previewContainer = document.getElementById('previewContainer');
    const controlsSection = document.getElementById('controlsSection');
    const processBtn = document.getElementById('processBtn');

    // Клик по области загрузки
    uploadArea.addEventListener('click', () => fileInput.click());
    
    // Drag and drop
    uploadArea.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadArea.classList.add('dragover');
    });
    
    uploadArea.addEventListener('dragleave', () => {
        uploadArea.classList.remove('dragover');
    });
    
    uploadArea.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadArea.classList.remove('dragover');
        if (e.dataTransfer.files.length) {
            fileInput.files = e.dataTransfer.files;
            fileInput.dispatchEvent(new Event('change'));
        }
    });
    
    // Выбор файла
    fileInput.addEventListener('change', handleFileSelect);
    
    function handleFileSelect(e) {
        if (!e.target.files.length) return;
        
        const file = e.target.files[0];
        
        // Проверка типа файла
        if (!CONFIG.allowedTypes.includes(file.type)) {
            alert('Пожалуйста, выберите изображение (JPG, PNG, GIF, BMP, WebP)');
            return;
        }
        
        // Проверка размера
        if (file.size > CONFIG.maxFileSize) {
            alert('Файл слишком большой! Максимум 20MB.');
            return;
        }
        
        const reader = new FileReader();
        reader.onload = function(e) {
            state.originalImage = e.target.result;
            state.originalFile = file;
            
            // Показываем изображение
            document.getElementById('originalImg').src = state.originalImage;
            document.getElementById('resultImg').src = state.originalImage;
            
            // Показываем интерфейс
            previewContainer.style.display = 'block';
            controlsSection.style.display = 'block';
            processBtn.disabled = false;
            
            // Скрываем результат
            document.getElementById('resultContainer').style.display = 'none';
            document.getElementById('downloadBtn').disabled = true;
            
            // Информация о файле
            const size = (file.size / 1024 / 1024).toFixed(2);
            
            // Получаем размеры изображения
            const img = new Image();
            img.onload = function() {
                document.getElementById('originalInfo').textContent = 
                    file.name + ' (' + size + ' MB, ' + img.width + '×' + img.height + ')';
                
                // Устанавливаем размеры
                document.getElementById('widthInput').value = img.width;
                document.getElementById('heightInput').value = img.height;
                state.settings.width = img.width;
                state.settings.height = img.height;
                
                // Сохраняем соотношение сторон
                const aspectRatio = img.width / img.height;
                const widthInput = document.getElementById('widthInput');
                const heightInput = document.getElementById('heightInput');
                const keepAspect = document.getElementById('keepAspect');
                
                widthInput.addEventListener('input', function() {
                    if (keepAspect.checked) {
                        const newWidth = parseInt(this.value) || img.width;
                        const newHeight = Math.round(newWidth / aspectRatio);
                        heightInput.value = newHeight;
                        state.settings.width = newWidth;
                        state.settings.height = newHeight;
                    } else {
                        state.settings.width = parseInt(this.value) || img.width;
                    }
                });
                
                heightInput.addEventListener('input', function() {
                    if (keepAspect.checked) {
                        const newHeight = parseInt(this.value) || img.height;
                        const newWidth = Math.round(newHeight * aspectRatio);
                        widthInput.value = newWidth;
                        state.settings.width = newWidth;
                        state.settings.height = newHeight;
                    } else {
                        state.settings.height = parseInt(this.value) || img.height;
                    }
                });
            };
            img.onerror = function() {
                document.getElementById('originalInfo').textContent = 
                    file.name + ' (' + size + ' MB)';
            };
            img.src = state.originalImage;
        };
        reader.readAsDataURL(file);
    }
}

// Инициализация фильтров
function initFilters() {
    const filtersContainer = document.getElementById('filtersContainer');
    
    // Загружаем фильтры с сервера
    fetch('/api/filters')
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                data.filters.forEach(filter => {
                    const button = document.createElement('button');
                    button.className = 'filter-btn';
                    button.innerHTML = filter.icon + ' ' + filter.name;
                    button.dataset.filter = filter.id;
                    
                    if (filter.id === 'none') {
                        button.classList.add('active');
                    }
                    
                    button.addEventListener('click', () => {
                        // Снимаем активность со всех кнопок
                        document.querySelectorAll('.filter-btn').forEach(btn => {
                            btn.classList.remove('active');
                        });
                        // Активируем текущую
                        button.classList.add('active');
                        state.settings.filter = filter.id;
                    });
                    
                    filtersContainer.appendChild(button);
                });
            }
        })
        .catch(error => {
            console.error('Ошибка загрузки фильтров:', error);
            // Запасной вариант
            const defaultFilters = [
                {id: 'none', name: 'Без фильтра', icon: '🔄'},
                {id: 'grayscale', name: 'Черно-белый', icon: '⚫'},
                {id: 'sepia', name: 'Сепия', icon: '🟤'},
                {id: 'invert', name: 'Инверсия', icon: '🔄'},
                {id: 'cool', name: 'Холодный', icon: '❄️'},
                {id: 'warm', name: 'Теплый', icon: '🔥'},
            ];
            
            defaultFilters.forEach(filter => {
                const button = document.createElement('button');
                button.className = 'filter-btn';
                button.innerHTML = filter.icon + ' ' + filter.name;
                button.dataset.filter = filter.id;
                
                if (filter.id === 'none') {
                    button.classList.add('active');
                }
                
                button.addEventListener('click', () => {
                    document.querySelectorAll('.filter-btn').forEach(btn => {
                        btn.classList.remove('active');
                    });
                    button.classList.add('active');
                    state.settings.filter = filter.id;
                });
                
                filtersContainer.appendChild(button);
            });
        });
}

// Инициализация элементов управления
function initControls() {
    // Поворот
    const rotateSlider = document.getElementById('rotateSlider');
    const rotateValue = document.getElementById('rotateValue');
    
    rotateSlider.addEventListener('input', function() {
        rotateValue.textContent = this.value + '°';
        state.settings.rotate = parseFloat(this.value);
    });
    
    // Быстрый поворот
    document.querySelectorAll('[data-rotate]').forEach(btn => {
        btn.addEventListener('click', function() {
            const angle = parseInt(this.dataset.rotate);
            rotateSlider.value = angle;
            rotateValue.textContent = angle + '°';
            state.settings.rotate = angle;
        });
    });
    
    // Отражение
    document.querySelectorAll('[data-flip]').forEach(btn => {
        btn.addEventListener('click', function() {
            document.querySelectorAll('[data-flip]').forEach(b => {
                b.classList.remove('active');
            });
            this.classList.add('active');
            state.settings.flip = this.dataset.flip;
        });
    });
    
    // Формат
    document.getElementById('formatSelect').addEventListener('change', function() {
        state.settings.format = this.value;
    });
    
    // Качество
    const qualitySlider = document.getElementById('qualitySlider');
    const qualityValue = document.getElementById('qualityValue');
    
    qualitySlider.addEventListener('input', function() {
        qualityValue.textContent = this.value + '%';
        state.settings.quality = parseInt(this.value);
    });
}

// Инициализация действий
function initActions() {
    const processBtn = document.getElementById('processBtn');
    const downloadBtn = document.getElementById('downloadBtn');
    const resetBtn = document.getElementById('resetBtn');
    const loading = document.getElementById('loading');
    const resultContainer = document.getElementById('resultContainer');
    
    // Обработка изображения
    processBtn.addEventListener('click', async () => {
        if (!state.originalFile) return;
        
        // Показываем индикатор загрузки
        loading.style.display = 'block';
        processBtn.disabled = true;
        
        try {
            const formData = new FormData();
            formData.append('image', state.originalFile);
            formData.append('filter', state.settings.filter);
            formData.append('rotate', state.settings.rotate.toString());
            formData.append('flip', state.settings.flip);
            formData.append('width', state.settings.width.toString());
            formData.append('height', state.settings.height.toString());
            formData.append('format', state.settings.format);
            formData.append('quality', state.settings.quality.toString());
            
            const response = await fetch('/api/process', {
                method: 'POST',
                body: formData
            });
            
            if (!response.ok) {
                throw new Error('Ошибка сервера: ' + response.status);
            }
            
            // Получаем результат
            const blob = await response.blob();
            state.processedImage = URL.createObjectURL(blob);
            
            // Показываем результат
            document.getElementById('resultImg').src = state.processedImage;
            
            // Информация о результате
            const processedSize = (blob.size / 1024 / 1024).toFixed(2);
            document.getElementById('resultInfo').textContent = 
                'Обработано (' + processedSize + ' MB)';
            
            // Активируем кнопку скачивания
            downloadBtn.disabled = false;
            resultContainer.style.display = 'block';
            
        } catch (error) {
            alert('Ошибка при обработке: ' + error.message);
            console.error(error);
        } finally {
            loading.style.display = 'none';
            processBtn.disabled = false;
        }
    });
    
    // Скачивание результата
    downloadBtn.addEventListener('click', () => {
        if (!state.processedImage) return;
        
        const a = document.createElement('a');
        a.href = state.processedImage;
        a.download = 'processed_image.' + state.settings.format;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    });
    
    // Сброс
    resetBtn.addEventListener('click', () => {
        // Сброс состояния
        state = {
            originalImage: null,
            processedImage: null,
            originalFile: null,
            settings: {
                filter: 'none',
                rotate: 0,
                flip: 'none',
                width: 800,
                height: 600,
                format: 'jpg',
                quality: 85
            }
        };
        
        // Сброс UI
        document.getElementById('originalImg').src = '';
        document.getElementById('resultImg').src = '';
        document.getElementById('previewContainer').style.display = 'none';
        document.getElementById('controlsSection').style.display = 'none';
        document.getElementById('resultContainer').style.display = 'none';
        document.getElementById('processBtn').disabled = true;
        document.getElementById('downloadBtn').disabled = true;
        document.getElementById('fileInput').value = '';
        document.getElementById('loading').style.display = 'none';
        
        // Сброс значений
        document.getElementById('rotateSlider').value = 0;
        document.getElementById('rotateValue').textContent = '0°';
        document.getElementById('widthInput').value = 800;
        document.getElementById('heightInput').value = 600;
        document.getElementById('qualitySlider').value = 85;
        document.getElementById('qualityValue').textContent = '85%';
        document.getElementById('formatSelect').value = 'jpg';
        
        // Сброс активных кнопок
        document.querySelectorAll('.filter-btn').forEach(btn => {
            btn.classList.remove('active');
            if (btn.dataset.filter === 'none') {
                btn.classList.add('active');
            }
        });
        
        document.querySelectorAll('[data-flip]').forEach(btn => {
            btn.classList.remove('active');
            if (btn.dataset.flip === 'none') {
                btn.classList.add('active');
            }
        });
    });
}`

	os.WriteFile("static/script.js", []byte(js), 0644)
}
