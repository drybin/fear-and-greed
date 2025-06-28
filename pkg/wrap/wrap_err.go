package wrap

import "github.com/ztrue/tracerr"

func Errorf(message string, args ...interface{}) error {
	return tracerr.Errorf(message, args...)
}
