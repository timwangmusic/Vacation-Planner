package utils

import (
	log "github.com/sirupsen/logrus"
	"runtime/debug"
)

const (
	LogPanic = iota
	LogFatal
	LogError
	LogWarning
	LogInfo
	LogDebug
	LogTrace
)

type Error struct {
	Err   error
	Level uint
}

func (error Error) Error() (res string) {
	res = error.Err.Error()
	return res
}

// LogErrorWithLevel logs error with severity if not nil
func LogErrorWithLevel(err error, level uint) bool {
	if err != nil {
		switch level {
		case LogPanic:
			log.Panic(err)
		case LogFatal:
			log.Fatal(err)
		case LogError:
			log.Error(err)
		case LogWarning:
			log.Warn(err)
		case LogInfo:
			log.Info(err)
		case LogDebug:
			log.Debug(err)
		case LogTrace:
			log.Trace(err)
		default:
			log.Error("No Level is provided for this error")
		}
		// print debug stack only if error level is higher than some threshold
		if level <= LogError {
			debug.PrintStack()
		}
		return true
	}
	return false
}
