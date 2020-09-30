package daemon

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strconv"
)

var (
	command = &Daemon{command: &cobra.Command{Use: Name()}}
)

// Command Set commands to your own running worker. After all,
// your own program will also need various parameters. If you implement this interface,
//SetCommand will be executed before startup, passing in the cobra.Command object, which can be saved for use.
type Command interface {
	SetCommand(cmd *cobra.Command)
}

func start(worker *Process) *cobra.Command {
	start := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("start %s", worker.worker.Name()),
		Run: func(cmd *cobra.Command, args []string) {
			isDaemon, err := cmd.Flags().GetBool("daemon")
			if err != nil {
				isDaemon = true
			}

			// If --daemon=false is passed in, the environment variable DAEMON will be directly written as true,
			// to allow the real program logic to run off the background.
			if !isDaemon {
				_ = os.Setenv(worker.DaemonTag, "true")
			}

			err = worker.Run()
			if err != nil {
				if err.Error() == "resource temporarily unavailable" {
					fmt.Println("resource temporarily unavailable")
					os.Exit(0)
				}
				panic(err)
			}
		},
	}

	start.PersistentFlags().BoolP("daemon", "d", true, "--daemon=false")
	return start
}

func stop(worker *Process) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: fmt.Sprintf("stop %s", worker.worker.Name()),
		Run: func(cmd *cobra.Command, args []string) {
			data, err := ioutil.ReadFile(worker.Pid.SaveFilename())
			if err != nil {
				if os.IsNotExist(err) {
					return
				}
				panic(err)
			}
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				panic(err)
			}
			process, err := os.FindProcess(pid)
			if err != nil {
				panic(err)
			}
			_ = process.Signal(SIGUSR1)
		},
	}
}

func restart(worker *Process) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: fmt.Sprintf("restart %s", worker.worker.Name()),
		Run: func(cmd *cobra.Command, args []string) {
			data, err := ioutil.ReadFile(worker.Pid.SaveFilename())
			if err != nil {
				if os.IsNotExist(err) {
					isDaemon, err := cmd.Flags().GetBool("daemon")
					if err != nil {
						isDaemon = true
					}

					if !isDaemon {
						_ = os.Setenv(worker.DaemonTag, "true")
					}

					err = worker.Run()
					if err != nil {
						panic(err)
					}
					return
				}
				panic(err)
			}
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				panic(err)
			}
			process, err := os.FindProcess(pid)
			if err != nil {
				panic(err)
			}
			_ = process.Signal(SIGUSR2)
		},
	}
}

// Daemon manager
type Daemon struct {
	command  *cobra.Command
	children map[string]*Daemon
	parent   *Daemon
	worker   *Process
}

// AddWorker add child exec process
// chainable call to generate multi-level commands.
// non-chained calls generate multiple sibling commands, but remember that sibling commands do not have the same name
func (daemon *Daemon) AddWorker(worker *Process) *Daemon {
	if daemon.children == nil {
		daemon.children = make(map[string]*Daemon)
	}

	child := &Daemon{command: &cobra.Command{Use: worker.worker.Name()}, parent: daemon}
	if _, ok := worker.worker.(Command); ok {
		worker.worker.(Command).SetCommand(child.command)
	}
	child.command.AddCommand(start(worker), stop(worker), restart(worker))
	daemon.command.AddCommand(child.command)
	daemon.children[worker.worker.Name()] = child
	return child
}

// GetParent get parent Daemon
func (daemon *Daemon) GetParent() *Daemon {
	return daemon.parent
}

// Register register main service, if not, you don't have to register.
func Register(worker *Process) {
	command.parent = nil
	command.worker = worker
	if _, ok := worker.worker.(Command); ok {
		worker.worker.(Command).SetCommand(command.command)
	}
	command.command.AddCommand(start(worker), stop(worker), restart(worker))
}

// GetCommand get main Daemon
func GetCommand() *Daemon {
	return command
}

// Run entry point
func Run() error {
	return command.command.Execute()
}

// Name get bin package file name
func Name() string {
	fileInfo, err := os.Stat(os.Args[0])
	if err != nil {
		return ""
	}
	return fileInfo.Name()
}
