package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"syscall"

	"github.com/kenretto/daemon"
	"github.com/spf13/cobra"
)

// HTTPServer http server example
type HTTPServer struct {
	http *http.Server
	cmd  *cobra.Command
}

// PidSavePath pid save path
func (httpServer *HTTPServer) PidSavePath() string {
	return "./"
}

// Name pid filename
func (httpServer *HTTPServer) Name() string {
	return "http"
}

// SetCommand get the cobra.Command object from daemon
func (httpServer *HTTPServer) SetCommand(cmd *cobra.Command) {
	// when you add parameters here, their parameters do not correspond to the start stop restart command of the service, such as this example service.
	// it corresponds to the sample service command, so the custom flag added here should be passed in before start
	cmd.PersistentFlags().StringP("test", "t", "yes", "")
	httpServer.cmd = cmd
}

// Start start web server
func (httpServer *HTTPServer) Start() {
	fmt.Println(httpServer.cmd.Flags().GetString("test"))
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Println("hello world")
		_, _ = writer.Write([]byte("hello world"))
	})
	httpServer.http = &http.Server{Handler: http.DefaultServeMux, Addr: ":9047"}
	_ = httpServer.http.ListenAndServe()
}

// Stop stop web server
func (httpServer *HTTPServer) Stop() error {
	fmt.Println("closing web server")
	err := httpServer.http.Shutdown(context.Background())
	fmt.Println("web server closed")
	return err
}

// Restart shut down the web service before restarting the http service
func (httpServer *HTTPServer) Restart() error {
	fmt.Println("closing web server")
	err := httpServer.Stop()
	return err
}

func main() {
	// Custom output file
	out, _ := os.OpenFile("./http.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	err, _ := os.OpenFile("./http_err.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)

	// Initialize a new running program
	proc := daemon.NewProcess(new(HTTPServer)).SetPipeline(nil, out, err)
	proc.On(syscall.SIGTERM, func() {
		fmt.Println("a custom signal")
	})
	// example: multi-level command service.
	// because the Command interface is implemented in the example here, there will be a situation where flag test does not exist. In fact, each worker should be unique.
	// do not share a worker object pointer
	daemon.GetCommand().AddWorker(proc).AddWorker(proc)
	// example: register main service
	daemon.Register(proc)

	// run
	if rs := daemon.Run(); rs != nil {
		log.Fatalln(rs)
	}
}
