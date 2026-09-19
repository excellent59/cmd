package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	log   *zap.Logger
	sugar *zap.SugaredLogger
	once  sync.Once
)

// Init инициализирует логгер. Вызывать один раз в начале main().
// env: "development" или "production"
func Init(env string) {
	once.Do(func() {
		// Настройка ротации логов (файл будет очищаться при достижении 100MB)
		rotatingWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   "logs/bot.log",
			MaxSize:    100,  // мегабайт
			MaxBackups: 5,    // хранить 5 старых файлов
			MaxAge:     30,   // дней
			Compress:   true, // сжимать старые файлы в .gz
		})

		var core zapcore.Core
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		if env == "production" {
			// В продакшене используем JSON для удобного парсинга (например, в Grafana Loki)
			encoderConfig = zap.NewProductionEncoderConfig()
			core = zapcore.NewCore(
				zapcore.NewJSONEncoder(encoderConfig),
				zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), rotatingWriter),
				zapcore.InfoLevel,
			)
		} else {
			// В разработке используем цветной консольный вывод + запись в файл
			encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			core = zapcore.NewCore(
				zapcore.NewConsoleEncoder(encoderConfig),
				zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), rotatingWriter),
				zapcore.DebugLevel,
			)
		}

		// AddCaller() добавляет имя файла и строку, откуда вызван лог
		// AddStacktrace() добавляет стек вызовов для ошибок уровня Error и выше
		log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
		sugar = log.Sugar()
	})
}

// Get возвращает удобный SugaredLogger для использования в коде
func Get() *zap.SugaredLogger {
	if sugar == nil {
		Init("development")
	}
	return sugar
}

// Sync безопасно закрывает логгер и сбрасывает буферы на диск.
// Обязательно вызывать через defer в main()
func Sync() {
	if log != nil {
		_ = log.Sync()
	}
}
