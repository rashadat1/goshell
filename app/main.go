package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)


func main() {
	for {
		_, err := fmt.Fprint(os.Stdout, "$ ")
		if err != nil {
			log.Println("Error writing shell intialization bytes to stdout: " + err.Error())
		}
		in := bufio.NewReader(os.Stdin)
		command, err := in.ReadString('\n')
		if err != nil {
			log.Println("Error reading string from standard in " + err.Error())
		}
		commandProcessor(command)
		// command contains a trailing \n byte so we slice out that last bit
	}	
}

func commandProcessor(input string) {
	commandParts := strings.Split(input, " ")
	commandName := strings.Trim(commandParts[0], "\r\n")
	if commandName == "exit" {
		if strings.Trim(commandParts[1], "\r\n") == "0" {
			os.Exit(0)
		}
	} else if commandName == "echo" {
		echoVal := strings.Join(commandParts[1:], " ")
		echoVal = strings.Trim(echoVal, "\r\n")
		fmt.Fprintln(os.Stdout, echoVal)
	} else {
		fmt.Fprintln(os.Stdout, input[:len(input) - 1] + ": command not found")
	}
	return
}
