package ipclogging

import (
	"github.com/igadmg/golang-ipc/ipcconfig"
	"log"
)

var (
	DoDebug   = ipcconfig.IpcDebugLogging
	DoLogging = true
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func PauseLogging() {
	DoLogging = false
}
func ContinueLogging() {
	DoLogging = true
}

func Print(v ...any) {
	if DoLogging {
		log.Print(append([]any{"INFO "}, v...)...)
	}
}
func Println(v ...any) {
	if DoLogging {
		log.Println(append([]any{"INFO "}, v...)...)
	}
}
func Printf(format string, v ...any) {
	if DoLogging {
		log.Printf("INFO  "+format, v...)
	}
}

func Debug(v ...any) {
	if DoLogging && DoDebug {
		log.Print(append([]any{"DEBUG"}, v...)...)
	}
}
func Debugln(v ...any) {
	if DoLogging && DoDebug {
		log.Println(append([]any{"DEBUG"}, v...)...)
	}
}
func Debugf(format string, v ...any) {
	if DoLogging && DoDebug {
		log.Printf("DEBUG "+format, v...)
	}
}

func Warn(v ...any) {
	log.Print(append([]any{"WARN "}, v...)...)
}
func Warnln(v ...any) {
	log.Println(append([]any{"WARN "}, v...)...)
}
func Warnf(format string, v ...any) {
	log.Printf("WARN "+format, v...)
}

func Status(v ...any) {
	log.Print(append([]any{"STATE "}, v...)...)
}
func Statusln(v ...any) {
	log.Println(append([]any{"STATE "}, v...)...)
}
func Statusf(format string, v ...any) {
	log.Printf("STATE "+format, v...)
}
