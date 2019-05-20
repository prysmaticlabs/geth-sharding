package logutil

import (
	"os"
	"strings"

	joonix "github.com/joonix/log"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

// WriterHook is a hook that writes logs of specified LogLevels to specified Writer
type WriterHook struct {
	LogLevels []logrus.Level
}

// Fire will be called when some logging function is called with current hook
// It will format log entry to string and write it to appropriate writer
func (hook *WriterHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	//simply call the file logger Println func after removing the new line char
	line = strings.TrimSuffix(line, "\n")
	fileLogger.Println(line)
	return err
}

// Levels define on which log levels this hook would trigger
func (hook *WriterHook) Levels() []logrus.Level {
	return hook.LogLevels
}

// File Logger insyance
var fileLogger = &logrus.Logger{
	Level: logrus.TraceLevel,
}

//ConfigurePersistentLogging starts a persistent file logger
func ConfigurePersistentLogging(logFileName string, logFileFormatName string) (bool, error) {
	logrus.WithField("logFileName", logFileName).Info("Logs will be made persistent")
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return false, err
	}
	fileLogger.SetOutput(f)

	//configure format if specified, othereise use the stdout logger's format
	switch logFileFormatName {
	case "text":
		formatter := new(prefixed.TextFormatter)
		formatter.TimestampFormat = "2006-01-02 15:04:05"
		formatter.FullTimestamp = true
		formatter.DisableColors = true
		fileLogger.SetFormatter(formatter)
		break
	case "fluentd":
		fileLogger.SetFormatter(&joonix.FluentdFormatter{})
		break
	case "json":
		fileLogger.SetFormatter(&logrus.JSONFormatter{})
		break
	default:
		logrus.Fatalf("must specifiy log file format when logging to persistent log file.")
	}

	logrus.Info("File logger initialized")
	//trigger writing to the log file on every stdout log write
	logrus.AddHook(&WriterHook{
		LogLevels: []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
			logrus.WarnLevel,
			logrus.InfoLevel,
			logrus.DebugLevel,
			logrus.TraceLevel,
		},
	})

	return true, nil
}
