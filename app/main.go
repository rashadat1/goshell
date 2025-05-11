package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const typeFound string = " is a shell builtin"

var shellBuiltIn []string = []string{"echo", "exit", "type"}

func main() {
	path := os.Getenv("PATH")
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
		commandProcessor(command, path)
		// command contains a trailing \n byte so we slice out that last bit
	}
}

func commandProcessor(input, path string) {
	commandParts := strings.Split(input, " ")
	for i := range commandParts {
		commandParts[i] = strings.Trim(commandParts[i], "\r\n ")
	}
	commandName := commandParts[0]
	directories := strings.Split(path, ":")

	if commandName == "exit" {
		if commandParts[1] == "0" {
			os.Exit(0)
		}
	} else if commandName == "echo" {
		echoVal := strings.Join(commandParts[1:], " ")
		echoVal = strings.Trim(echoVal, "\r\n")
		fmt.Fprintln(os.Stdout, echoVal)
		return
	} else if commandName == "type" {
		if len(commandParts) <= 1 {
			fmt.Fprintln(os.Stdout, "type takes two arguments but none were given")
			return
		}
		typeArg := strings.Join(commandParts[1:], " ")
		if slices.Contains(shellBuiltIn, typeArg) {
			fmt.Fprintln(os.Stdout, typeArg+" is a shell builtin")
			return
		}
		for i := range len(directories) {
			pathToExecutable, _ := checkForExecutable(directories[i], typeArg)
			if pathToExecutable != "" {
				fmt.Fprintln(os.Stdout, typeArg+" is "+pathToExecutable)
				return
			}
		}
		fmt.Fprintln(os.Stdout, typeArg+": not found")
		return
	} else {
		for i := range len(directories) {
			pathToExecutable, _ := checkForExecutable(directories[i], commandName)
			if pathToExecutable != "" {
				cmd := exec.Command(pathToExecutable, commandParts[1:]...)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Args[0] = commandName
				err := cmd.Run()
				if err != nil {
					fmt.Fprintln(os.Stdout, "Error running command: "+err.Error())
				}
				return
			}
		}
		fmt.Fprintln(os.Stdout, input[:len(input)-1]+": command not found")
	}
	return
}

func checkForExecutable(path, command string) (string, error) {
	c, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	for _, entry := range c {
		if entry.Name() == command {
			return path + "/" + entry.Name(), nil
		}
	}
	return "", nil
}
