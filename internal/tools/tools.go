package tools

import (
	"errors"
	"io/ioutil"
)

func writeFile(path string, content string) error {
	return ioutil.WriteFile(path, []byte(content), 0644)
}
