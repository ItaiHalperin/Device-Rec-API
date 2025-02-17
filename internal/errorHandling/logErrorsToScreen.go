package errorHandling

import (
	"fmt"
	"log"
)

type LogParams struct {
	DeviceName string
	Function   string
	ErrorMsg   string
	IsCtxError bool
}

func LogErrorToScreen(logParams LogParams, err error) {
	var str string
	if logParams.IsCtxError {
		str = fmt.Sprintf("in %v, stopping process: %v", logParams.Function, err)
	} else if logParams.DeviceName == "" {
		str = fmt.Sprintf("in %v, %v: %v", logParams.Function, logParams.ErrorMsg, err)
	} else {
		str = fmt.Sprintf("in %v (device: %v), %v: %v", logParams.Function, logParams.DeviceName,
			logParams.ErrorMsg, err)
	}
	log.Printf(str)
}
