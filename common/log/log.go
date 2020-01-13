package log

import "log"

var debugLogEnabled bool

// EnableDebugLog can enable or disable debug logs.
func EnableDebugLog(enable bool) {
	debugLogEnabled = enable
}

// DebugLogEnabled returns true if the debug log is enabled.
func DebugLogEnabled() bool {
	return debugLogEnabled
}

// Debugf is the wrapper of the standard log.Printf, but printed only when the debug log is enabled.
func Debugf(format string, v ...interface{}) {
	if debugLogEnabled {
		log.Printf(format, v...)
	}
}

// Fatal is the wrapper of the standard log.Fatal.
func Fatal(v ...interface{}) {
	log.Fatal(v...)
}

// Fatalf is the wrapper of the standard log.Fatalf.
func Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}

// Printf is the wrapper of the standard log.Printf.
func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}
