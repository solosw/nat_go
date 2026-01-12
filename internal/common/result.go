package common

type Result struct {
	Code    int         `json:"code"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(data interface{}) Result {

	return Result{200, true, "", data}
}

func SuccessWithMessage(data interface{}, message string) Result {
	return Result{200, true, message, data}
}

func Error(code int, message string) Result {
	return Result{code, false, message, nil}
}
func ErrorWithData(data interface{}, code int, message string) Result {
	return Result{code, true, message, data}
}
