package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ubuntu/face-detection-demo/comm"
	"github.com/ubuntu/face-detection-demo/datastore"
	"github.com/ubuntu/face-detection-demo/detection"
	"github.com/ubuntu/face-detection-demo/messages"
)

var (
	wg       *sync.WaitGroup
	shutdown chan interface{}
	rootdir  string
)

func main() {
	var err error

	if rootdir, err = filepath.Abs(path.Join(filepath.Dir(os.Args[0]), "..")); err != nil {
		log.Fatal(err)
	}
	datadir := os.Getenv("SNAP_DATA")
	if datadir == "" {
		datadir = rootdir
	}

	datastore.LoadSettings(datadir)

	wg = new(sync.WaitGroup)
	shutdown = make(chan interface{})

	// handle user generated stop requests
	userstop := make(chan os.Signal)
	signal.Notify(userstop, syscall.SIGINT, syscall.SIGTERM)

	actions := make(chan *messages.Action, 2)

	comm.StartSocketListener(actions, shutdown, wg)

	// starts camera if it was already started last time
	if datastore.GetDetection() {
		detection.StartCameraDetect(rootdir, shutdown, wg)
	}

mainloop:
	for {
		fmt.Println("main loop")
		select {
		case action := <-actions:
			fmt.Println("new action received")
			if processaction(action) {
				break mainloop
			}
		case <-userstop:
			quit()
			break mainloop
		case <-time.After(5 * time.Second):
			fmt.Println("timeout")
			var foo *messages.Action
			if datastore.GetDetection() {
				foo = &messages.Action{
					DrawMode:    messages.Action_MODE_UNCHANGED,
					CameraState: messages.Action_CAMERA_DISABLE,
					QuitServer:  false,
				}
				fmt.Println("Switch camera off")
			} else {
				foo = &messages.Action{
					DrawMode:    messages.Action_MODE_UNCHANGED,
					CameraState: messages.Action_CAMERA_ENABLE,
					QuitServer:  false,
				}
				fmt.Println("Switch camera on")
			}
			actions <- foo
		}
	}

	wg.Wait()
}

// process action and return true if we need to quit (exit mainloop)
func processaction(action *messages.Action) bool {
	if action.CameraState == messages.Action_CAMERA_ENABLE {
		detection.StartCameraDetect(rootdir, shutdown, wg)
		fmt.Println("Received camera on")
	} else if action.CameraState == messages.Action_CAMERA_DISABLE {
		detection.EndCameraDetect()
		fmt.Println("Received camera off")
	}
	if action.QuitServer {
		quit()
		return true
	}
	return false
}

func quit() {
	fmt.Println("quit server")
	// signal all main goroutines to exits
	close(shutdown)
}
