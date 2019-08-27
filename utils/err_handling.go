package utils

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"runtime/debug"
)
const(
	LOG_PANIC = iota
	LOG_FATAL
	LOG_ERROR
	LOG_WARNING
	LOG_INFO
	LOG_DEBUG
	LOG_TRACE
)
type UtilsError struct{
	Err error
	level uint
}
func (this UtilsError) Error() (res string){
	res = this.Err.Error()
	return res
}
func CheckErrImmediate(err error, level uint){
	if err != nil{
		switch level {
		case LOG_PANIC :
			log.Panic(err)
		case LOG_FATAL:
			log.Fatal(err)
		case LOG_ERROR:
			log.Error(err)
		case LOG_WARNING:
			log.Warn(err)
		case LOG_INFO:
			log.Info(err)
		case LOG_DEBUG:
			log.Debug(err)
		case LOG_TRACE:
			log.Trace(err)
		default:
			log.Error("No level is provided for this error")
		}
	} else {
		log.Error("No Error is raised")
	}
	debug.PrintStack()
}
func CheckErr(err UtilsError){
	if err.Err != nil{
		switch err.level {
		case LOG_PANIC :
			log.Panic(err.Err)
		case LOG_FATAL:
			log.Fatal(err.Err)
		case LOG_ERROR:
			log.Error(err.Err)
		case LOG_WARNING:
			log.Warn(err.Err)
		case LOG_INFO:
			log.Info(err.Err)
		case LOG_DEBUG:
			log.Debug(err.Err)
		case LOG_TRACE:
			log.Trace(err.Err)
		default:
			log.Error("No level is provided for this error")
		}
	} else {
		log.Error("No Error is raised")
	}
}
func GenerateErr(errstring string, level uint) (err UtilsError){
	err = UtilsError{errors.New(errstring), level}
	debug.PrintStack()
	return
}
