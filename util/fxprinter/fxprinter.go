package fxprinter

import (
	"github.com/pingcap/log"
	"go.uber.org/fx"
)

type DebugLogPrinter struct{}

func (p DebugLogPrinter) Printf(m string, args ...interface{}) {
	log.S().Debugf(m, args...)
}

func NewDebugLogPrinter() fx.Printer {
	return DebugLogPrinter{}
}
