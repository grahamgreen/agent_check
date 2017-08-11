package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	systemstat "bitbucket.org/bertimus9/systemstat"
)

//CommandString string with a read/write mutex
type CommandString struct {
	sync.RWMutex
	cmd         string
	allowedCMDs map[string]string
}

//NewCMDS returns a new command string
func NewCMDS() *CommandString {
	aCMDs := make(map[string]string)
	aCMDs["READY"] = "READY"
	aCMDs["DRAIN"] = "DRAIN"
	aCMDs["MAINT"] = "MAINT"
	aCMDs["DOWN"] = "DOWN"
	aCMDs["FAILED"] = "FAILED"
	aCMDs["STOPPED"] = "STOPPED"
	aCMDs["UP"] = "UP"
	return &CommandString{
		cmd:         "UP",
		allowedCMDs: aCMDs,
	}
}

//Set sets the command strings value
func (cs *CommandString) Set(value string) string {
	cs.Lock()
	defer cs.Unlock()

	uv := strings.ToUpper(value)
	if cmd, ok := cs.allowedCMDs[uv]; ok {
		cs.cmd = cmd
		return cmd + " OK"
	}
	return "NOT SET"
}

//Get gets the command strings value
func (cs *CommandString) Get() string {
	cs.RLock()
	defer cs.RUnlock()

	return cs.cmd
}

func getIdle() (out int) {
	sample1 := systemstat.GetCPUSample()
	time.Sleep(100 * time.Millisecond)
	sample2 := systemstat.GetCPUSample()
	avg := systemstat.GetSimpleCPUAverage(sample1, sample2)
	idlePercent := avg.IdlePct
	return int(idlePercent)
}

func handleTalk(conn net.Conn, cmd *CommandString) {
	defer conn.Close()
	idle := strconv.Itoa(getIdle())
	commandStr := cmd.Get()
	io.WriteString(conn, commandStr+" "+idle+"% \n")
	return
}

func handleListen(conn net.Conn, cmd *CommandString) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	line = strings.Replace(line, "\n", "", -1)
	rtn := cmd.Set(line)
	conn.Write([]byte(rtn + "\n"))
	return
}

func talk(ln net.Listener, cmd *CommandString) {
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("there was an error:", err)
			break
		}
		go handleTalk(conn, cmd)
	}
}

func listen(ln net.Listener, cmd *CommandString) {
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("there was an error:", err)
			break
		}
		go handleListen(conn, cmd)
	}

}

func main() {
	listenPort := os.Getenv("AC_LISTEN_PORT")
	if listenPort == "" {
		fmt.Fprintf(os.Stderr, "AC LISTEN PORT not set\n")
		os.Exit(1)
	}
	talkPort := os.Getenv("AC_TALK_PORT")
	if talkPort == "" {
		fmt.Fprintf(os.Stderr, "AC TALK PORT not set\n")
		os.Exit(1)
	}
	cmd := NewCMDS()

	lp := ":" + listenPort
	ln, err := net.Listen("tcp", lp)
	if err != nil {
		log.Fatalln("there was an error:", err)
	}
	go talk(ln, cmd)

	tp := "localhost:" + talkPort
	ln2, err := net.Listen("tcp", tp)
	if err != nil {
		log.Fatalln("there was an error:", err)
	}
	go listen(ln2, cmd)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	s := <-c
	log.Println("exiting on:", s)
}
