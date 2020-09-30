package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Pid The process id information and process pid file descriptors that are mainly recorded here
type Pid struct {
	ServicesName string   // service name, not process name
	SavePath     string   // pid save path
	Pid          int      // pid num
	File         *os.File // file
}

// SaveFilename Get the path where the pid is saved
func (pid Pid) SaveFilename() string {
	path, err := filepath.Abs(pid.SavePath)
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("%s/%s.pid", path, pid.ServicesName)
}

// Save save pid
func (pid Pid) Save() error {
	var err error
	pid.File, err = write(pid.SaveFilename(), strconv.Itoa(pid.Pid))
	return err
}

// Remove Close the file descriptor and delete the pid file
func (pid Pid) Remove() {
	_ = pid.File.Close()
	_ = os.Remove(pid.SaveFilename())
}
