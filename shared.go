package ipc

import "errors"

// checks the name passed into the start function to ensure it's ok/will work.
func ipcNameValidate(ipcName string) error {
	if len(ipcName) == 0 {
		return errors.New("ipcName cannot be an empty string")
	}
	return nil
}
