package utils

import (
	"github.com/sirupsen/logrus"
	"os"
)

type Logger struct {
	*logrus.Logger
}

func InitLogger() *Logger {
	logger := logrus.New()

	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		DisableColors:   false,
		ForceColors:     true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logger.SetLevel(logrus.DebugLevel)

	return &Logger{logger}
}
