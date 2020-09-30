package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

const (
	// EnvName Identify the name of the environment variable that is the child process.
	// A simple method is to set an environment variable so that the program can determine whether it is created by its own parent process after getting it.
	EnvName = "DAEMON"
)

// Worker The interface that the working program must implement
type Worker interface {
	// PidSavePath pid file save path
	PidSavePath() string
	// Name pid file name
	Name() string
	// Start Program startup entry method
	Start()
	// Stop Program stop handle
	Stop() error
	// Restart Program restart handle
	Restart() error
}

type (
	// system signal handlers
	signalHandlers map[os.Signal]func()
	// Process a service process info
	Process struct {
		Pipeline       [3]*os.File // input/output pipe, 0->input, 1->output, 2->err
		Pid            *Pid        // pid pid info
		worker         Worker      // worker
		DaemonTag      string
		SignalHandlers signalHandlers // signal handlers
	}
)

// Listen listen all system signals
func (handlers signalHandlers) Listen() {
	var sig = make(chan os.Signal)
	signal.Notify(sig)
	for {
		received := <-sig
		if handler, ok := handlers[received]; ok {
			handler()
		}
	}
}

// NewProcess create a process instance with Worker
func NewProcess(worker Worker) *Process {
	process := &Process{
		Pipeline: [3]*os.File{os.Stdin, os.Stdout, os.Stderr},
		Pid: &Pid{
			ServicesName: worker.Name(),
			SavePath:     worker.PidSavePath(),
			Pid:          os.Getpid(),
		},
		worker:    worker,
		DaemonTag: EnvName,
	}
	process.registerDefaultInterruptHandle()
	process.registerDefaultStopHandle()
	process.registerDefaultRestartHandle()
	return process
}

// SetPipeline set standard i/o pipeline, 0 -> stdin(generally give up directly, you can send nil), 1 -> stdout, 2 -> stderr
// of course, you can choose not to set it.
func (process *Process) SetPipeline(pipes ...*os.File) *Process {
	if len(pipes) > 3 {
		pipes = pipes[0:3]
	}
	for index, pipe := range pipes {
		process.Pipeline[index] = pipe
	}
	return process
}

// SetDaemonTag custom DAEMON env name
func (process *Process) SetDaemonTag(name string) *Process {
	process.DaemonTag = name
	return process
}

// On register the signal handling method of the custom child process. The method registered here is actually running on the child process.
// The real program logic runs in a co-program of the child process, and the signal monitoring method of the main co-program running of the child process
func (process *Process) On(signal os.Signal, fn func()) {
	if process.SignalHandlers == nil {
		process.SignalHandlers = make(signalHandlers)
	}
	process.SignalHandlers[signal] = fn
}

// monitor interrupt signal operation
func (process *Process) registerDefaultInterruptHandle() {
	process.On(os.Interrupt, func() {
		err := process.worker.Stop()
		if err != nil {
			_, _ = process.Pipeline[1].WriteString(err.Error())
		}
		process.Pid.Remove()
		os.Exit(0)
	})
}

// register the default stop method and listen for USR1 signals
func (process *Process) registerDefaultStopHandle() {
	process.On(SIGUSR1, func() {
		err := process.worker.Stop()
		if err != nil {
			_, _ = process.Pipeline[1].WriteString(err.Error())
		}
		process.Pid.Remove()
		os.Exit(0)
	})
}

// register the default restart method and listen for USR2 signals
func (process *Process) registerDefaultRestartHandle() {
	process.On(SIGUSR2, func() {
		process.Pid.Remove()
		var done = make(chan bool)
		go func() {
			err := process.worker.Restart()
			if err != nil {
				_, _ = process.Pipeline[1].WriteString(err.Error())
			}
			done <- true
		}()
		_ = os.Unsetenv(process.DaemonTag)
		err := process.Run()
		if err != nil {
			_, _ = process.Pipeline[1].WriteString(err.Error())
		}
		<-done
		os.Exit(0)
	})
}

// IsChild To determine whether it is started in a child process, according to the environment variable DAEMON
func (process *Process) IsChild() bool {
	return os.Getenv(process.DaemonTag) == "true"
}

// Run Run the program, the main logic runs in the cooperative program, and the main cooperative program runs the system signal listener.
func (process *Process) Run() error {
	if process.IsChild() {
		if err := process.Pid.Save(); err != nil {
			return err
		}
		go process.worker.Start()
		process.SignalHandlers.Listen()
		return nil
	}

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=true", process.DaemonTag))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = process.Pipeline[0], process.Pipeline[1], process.Pipeline[2]

	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Process.Release()

}
