// Copyright 2020 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

// logLocation is the path to the location of the SuperTuxKart log file
const logLocation = "/.config/supertuxkart/config-0.10/server_config.log"

// main intercepts the log file of the SuperTuxKart gameserver and uses it
// to determine if the game server is ready or not.
func main() {
	log.SetPrefix("[wrapper] ")
	input := flag.String("i", "", "the command and arguments to execute the server binary")

	log.Println("Starting wrapper for SuperTuxKart")
	log.Printf("Command being run for SuperTuxKart server: %s \n", *input)

	cmdString := strings.Split(*input, " ")
	command, args := cmdString[0], cmdString[1:]

	cmd := exec.Command(command, args...) // #nosec
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		log.Fatalf("error starting cmd: %v", err)
	}

	// SuperTuxKart refuses to output to foreground, so we're going to
	// poll the server log.
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("could not get home dir: %v", err)
	}

	t := &tail.Tail{}
	// Loop to make sure the log has been created. Sometimes it takes a few seconds
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)

		t, err = tail.TailFile(path.Join(home, logLocation), tail.Config{Follow: true})
		if err != nil {
			log.Print(err)
			continue
		} else {
			break
		}
	}
	defer t.Cleanup()
	log.Fatal("tail ended")
}


// handleLogLine compares the log line to a series of regexes to determine if any action should be taken.
// TODO: This could probably be handled better with a custom type rather than just (string, *string)
func handleLogLine(line string) (string, *string) {
	// The various regexes that match server lines
	playerJoin := regexp.MustCompile(`ServerLobby: New player (.+) with online id [0-9][0-9]?`)
	playerLeave := regexp.MustCompile(`ServerLobby: (.+) disconnected$`)
	noMorePlayers := regexp.MustCompile(`STKHost.+There are now 0 peers\.$`)
	serverStart := regexp.MustCompile(`Listening has been started`)

	// Start the server
	if serverStart.MatchString(line) {
		log.Print("server ready")
		return "READY", nil
	}

	// Player tracking
	if playerJoin.MatchString(line) {
		matches := playerJoin.FindSubmatch([]byte(line))
		player := string(matches[1])
		log.Printf("Player %s joined\n", player)
		return "PLAYERJOIN", &player
	}
	if playerLeave.MatchString(line) {
		matches := playerLeave.FindSubmatch([]byte(line))
		player := string(matches[1])
		log.Printf("Player %s disconnected", player)
		return "PLAYERLEAVE", &player
	}

	// All the players left, send a shutdown
	if noMorePlayers.MatchString(line) {
		log.Print("server has no more players. shutting down")
		return "SHUTDOWN", nil
	}
	return "", nil
}
