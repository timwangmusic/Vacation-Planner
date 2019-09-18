package utils

import (
	"errors"
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

func CheckErrImmediate(err error, level uint) {
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
		debug.PrintStack()
	}

}

func CheckErr(err Error) {
	if err.Err != nil {
		switch err.Level {
		case LogPanic:
			log.Panic(err.Err)
		case LogFatal:
			log.Fatal(err.Err)
		case LogError:
			log.Error(err.Err)
		case LogWarning:
			log.Warn(err.Err)
		case LogInfo:
			log.Info(err.Err)
		case LogDebug:
			log.Debug(err.Err)
		case LogTrace:
			log.Trace(err.Err)
		default:
			log.Error("No Level is provided for this error")
		}
	} else {
		log.Error("No Error is raised")
	}
}

func GenerateErr(errString string, level uint) (err Error) {
	err = Error{errors.New(errString), level}
	debug.PrintStack()
	return
}
