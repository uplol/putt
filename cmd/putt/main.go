package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jubnzv/go-tmux"
)

type Process struct {
	Command string `json:"command"`
}

type ProcFile map[string]*Process

var procLineRe = regexp.MustCompile(`(.*): (.*)`)
var paneLineRe = regexp.MustCompile(`(\d): \[(\d+x\d+)\] \[(.*)] %(\d+)`)

func main() {
	path := "Procfile"

	if len(os.Args) == 1 {
		if _, err := os.Stat(path); err == os.ErrNotExist {
			fmt.Printf("Usage: putt <puttfile>\n")
			os.Exit(1)
			return
		}
	} else {
		path = os.Args[1]
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}
	defer file.Close()

	procFile := make(ProcFile)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		match := procLineRe.FindAllStringSubmatch(line, -1)
		if len(match) == 0 {
			fmt.Printf("Error: invalid line '%s'\n", line)
			os.Exit(1)
			return
		}

		name := match[0][1]
		procFile[name] = &Process{
			Command: match[0][2],
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	tmuxServer := os.Getenv("TMUX")
	if tmuxServer == "" {
		fmt.Printf("Your not running this from within a tmux session\n")
		os.Exit(1)
		return
	}

	tmuxPane := os.Getenv("TMUX_PANE")
	if tmuxPane == "" {
		fmt.Printf("Your not running this from within a tmux pane\n")
		os.Exit(1)
		return
	}

	tmuxPaneId, err := strconv.ParseInt(tmuxPane[1:], 10, 64)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	server := tmux.Server{}
	name, err := tmux.GetAttachedSessionName()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	sessions, err := server.ListSessions()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	var session tmux.Session
	found := false
	for _, i := range sessions {
		if i.Name == name {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("Failed to find active tmux session in server\n")
		os.Exit(1)
		return
	}

	panes, err := session.ListPanes()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	found = false
	var pane tmux.Pane
	for _, i := range panes {
		if i.ID == int(tmuxPaneId) {
			pane = i
			found = true
		}
	}

	if !found {
		fmt.Printf("Failed to find active tmux pane in session\n")
		os.Exit(1)
		return
	}

	target := pane.SessionName + ":" + pane.WindowName

	currentPanesRaw, _, err := tmux.RunCmd([]string{"list-panes", "-t", target})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	if len(strings.Split(currentPanesRaw, "\n")) > 2 {
		fmt.Printf("Too many tmux panes in the current window\n")
		os.Exit(1)
		return
	}

	for _, process := range procFile {
		tmux.RunCmd([]string{"split-window", "-t", target, "-h"})
		time.Sleep(time.Millisecond * 50)
		tmux.RunCmd([]string{"send-keys", "-t", target, process.Command + ""})
		tmux.RunCmd([]string{"send-keys", "-t", target, "Enter"})
	}

	tmux.RunCmd([]string{"select-layout", "-t", target, "tiled"})

	currentPanesRaw, _, err = tmux.RunCmd([]string{"list-panes", "-t", target})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	paneIds := []string{}
	for _, pane := range strings.Split(currentPanesRaw, "\n") {
		if pane == "" {
			continue
		}

		matches := paneLineRe.FindAllStringSubmatch(pane, -1)
		if len(matches) != 1 {
			fmt.Printf("Invalid pane: %v\n", pane)
			os.Exit(1)
			return
		}

		paneIds = append(paneIds, matches[0][4])
	}

	tmux.RunCmd([]string{"select-pane", "-t", "%" + paneIds[0]})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			for _, paneId := range paneIds[1:] {
				tmux.RunCmd([]string{"kill-pane", "-t", "%" + paneId})
			}
			os.Exit(0)
		}
	}()

	stdinScanner := bufio.NewScanner(os.Stdin)
	for stdinScanner.Scan() {
	}

	if stdinScanner.Err() != nil {
		fmt.Printf("Error: %v", stdinScanner.Err())
		os.Exit(1)
		return
	}
}
