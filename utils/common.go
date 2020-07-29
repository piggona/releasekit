package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	simplejson "github.com/bitly/go-simplejson"
)

func CheckArgs(arg ...string) {
	if len(os.Args) < len(arg)+1 {
		Warning("Usage: %s %s", os.Args[0], strings.Join(arg, " "))
	}
}

func CheckIfError(err error) {
	if err == nil {
		return
	}
	fmt.Printf("\x1b[31;1m%s\x1b[0m\n", fmt.Sprintf("error: %s", err))
	os.Exit(1)
}

func Info(format string, args ...interface{}) {
	fmt.Printf("\x1b[34;1m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

func Warning(format string, args ...interface{}) {
	fmt.Printf("\x1b[36;1m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

func ReadJSON(path string) (*simplejson.Json, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p, err := simplejson.NewJson(content)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
