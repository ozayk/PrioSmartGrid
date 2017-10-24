// Implements a basic control program for managing a
// Prio deployment on EC2.
package main

import (
	"fmt"
	"log"
	"os"

	"../config"
)

const LOCAL_GOBIN = "/usr/local/go/bin"
const EC2_USER = "root"
const EC2_KEY = "/root/PrioAWS.pem"

var sshOptions []string

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %v <cfg_file> <command>", os.Args[0])
	}

	cfg := config.LoadFile(os.Args[1])

	switch os.Args[2] {
	case "kill":
		runKill(cfg)
	case "killc":
		runKillClients(cfg)
	case "start":
		runStart(cfg)
	case "startd":
		runStartDummy(cfg)
	case "startc":
		runStartClients(cfg)
	case "startdc":
		runStartDummyClients(cfg)
	case "logs":
		runLogs(cfg)
	case "copy":
		runCopy(cfg)
	case "copycfg":
		runCopyConfig(cfg)
	case "rmlogs":
		runRmlogs(cfg)

	default:
		log.Fatal("Unrecognized command.")
	}
}

func init() {
	sshOptions = []string{
		"-o", fmt.Sprintf("User=%v", EC2_USER),
		"-o", "StrictHostKeyChecking=no",
		"-i", EC2_KEY}
}
