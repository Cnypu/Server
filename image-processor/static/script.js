// Конфигурация
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
}