package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

type ProcessRequest struct {
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Quality    int      `json:"quality"`
	Format     string   `json:"format"`
	Filter     string   `json:"filter"`
	Operations []string `json:"operations"`
}

func main() {
	// Создаем папку для загрузок
	os.MkdirAll("uploads", 0755)
	os.MkdirAll("static", 0755)

	// Статические файлы (HTML, CSS, JS)
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	// API для обработки изображений
	http.HandleFunc("/process", processImageHandler)

	// Старт сервера
	fmt.Println("🚀 Сервер запущен на http://localhost:8080")
	fmt.Println("📁 Загрузите изображение через веб-интерфейс")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	}
}

func processImageHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if r.Method != "POST" {
		http.Error(w, "Только POST метод", http.StatusMethodNotAllowed)
		return
	}

	// Парсим multipart форму
	err := r.ParseMultipartForm(50 << 20) // 50 MB
	if err != nil {
		sendError(w, "Ошибка парсинга формы", http.StatusBadRequest)
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("image")
	if err != nil {
		sendError(w, "Файл не найден", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Получаем параметры
	width, _ := strconv.Atoi(r.FormValue("width"))
	height, _ := strconv.Atoi(r.FormValue("height"))
	quality, _ := strconv.Atoi(r.FormValue("quality"))
	if quality == 0 {
		quality = 85
	}
	format := r.FormValue("format")
	if format == "" {
		format = "jpg"
	}
	filter := r.FormValue("filter")

	// Временно сохраняем файл
	tempPath := filepath.Join("uploads", fmt.Sprintf("temp_%d_%s", time.Now().Unix(), header.Filename))
	tempFile, err := os.Create(tempPath)
	if err != nil {
		sendError(w, "Ошибка создания временного файла", http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempPath) // Удаляем временный файл

	io.Copy(tempFile, file)

	// Обрабатываем изображение
	processedData, err := processImage(tempPath, width, height, quality, format, filter)
	if err != nil {
		sendError(w, fmt.Sprintf("Ошибка обработки: %v", err), http.StatusInternalServerError)
		return
	}

	// Отправляем результат
	w.Header().Set("Content-Type", getContentType(format))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"processed.%s\"", format))
	w.Write(processedData)

	// Логирование
	elapsed := time.Since(startTime)
	fmt.Printf("[%s] Обработан файл: %s -> %s (%dx%d) за %v\n",
		time.Now().Format("15:04:05"),
		header.Filename,
		format,
		width, height,
		elapsed)
}

func processImage(filepath string, width, height, quality int, format, filter string) ([]byte, error) {
	// Открываем изображение
	img, err := imaging.Open(filepath)
	if err != nil {
		return nil, err
	}

	// Сохраняем оригинальные размеры
	origBounds := img.Bounds()
	origWidth := origBounds.Dx()
	origHeight := origBounds.Dy()

	// Если не указаны размеры, используем оригинальные
	if width <= 0 {
		width = origWidth
	}
	if height <= 0 {
		height = origHeight
	}

	// Изменяем размер
	if width != origWidth || height != origHeight {
		img = imaging.Resize(img, width, height, imaging.Lanczos)
	}

	// Применяем фильтр
	switch filter {
	case "grayscale":
		img = imaging.Grayscale(img)
	case "sepia":
		img = applySepia(img)
	case "blur":
		img = imaging.Blur(img, 3.0)
	case "invert":
		img = imaging.Invert(img)
	case "brightness":
		img = imaging.AdjustBrightness(img, 0.2)
	}

	// Сохраняем в буфер
	var buf bytes.Buffer

	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality))
	case "png":
		err = imaging.Encode(&buf, img, imaging.PNG)
	case "webp":
		// Для WebP нужна отдельная библиотека
		err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality))
	case "gif":
		// Для GIF тоже нужна отдельная библиотека
		err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality))
	default:
		err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality))
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func applySepia(img image.Image) image.Image {
	// Простая реализация сепии
	dst := imaging.Clone(img)
	bounds := dst.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := dst.At(x, y).RGBA()

			// Формула сепии
			tr := float64(r>>8)*0.393 + float64(g>>8)*0.769 + float64(b>>8)*0.189
			tg := float64(r>>8)*0.349 + float64(g>>8)*0.686 + float64(b>>8)*0.168
			tb := float64(r>>8)*0.272 + float64(g>>8)*0.534 + float64(b>>8)*0.131

			// Ограничение значений
			tr = min(255, tr)
			tg = min(255, tg)
			tb = min(255, tb)

			dst.Set(x, y, color.RGBA{
				R: uint8(tr),
				G: uint8(tg),
				B: uint8(tb),
				A: uint8(a >> 8),
			})
		}
	}

	return dst
}

func getContentType(format string) string {
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"success": false,
	})
}
