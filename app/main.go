package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const typeFound string = " is a shell builtin"

var shellBuiltIn []string = []string{"echo", "exit", "type", "pwd", "cd"}
var escapeOptionsDoubleQuoted []rune = []rune{'\\', '$', '"', ' '}
var escapeOptionUnquoted []rune = []rune{'\\', '$', '"', ' ', '\''}

func main() {
	PATH := os.Getenv("PATH")
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
		commandProcessor(command, PATH)
	}
}

func commandProcessor(input, PATH string) {
	commandParts := strings.Split(input, " ")
	for i := range commandParts {
		commandParts[i] = strings.Trim(commandParts[i], "\r\n ")
	}
	commandName := commandParts[0]
	directories := strings.Split(PATH, ":")
	// default stdOut and stdErr output locations
	outputFilePath := ""
	errFilePath := ""
	outputWriter := os.Stdout
	errWriter := os.Stdout

	// create an argParts without the redirection symbol

	outputFilePath, errFilePath = parseOutputRedirect(input)
	os.MkdirAll(filepath.Dir(outputFilePath), 0755)
	os.MkdirAll(filepath.Dir(errFilePath), 0755)

	// remove redirection so this is not interpreted as a command argument
	removedRedirect := removeRedirection(input)
	cmdParsed, argsParts := parseCommandArgs(removedRedirect)

	commandName = cmdParsed
	argsString := strings.Join(argsParts, " ")

	if outputFilePath != "" {
		outputWriter, _ = os.OpenFile(outputFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		defer outputWriter.Close()
	}
	if errFilePath != "" {
		errWriter, _ = os.OpenFile(errFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		defer errWriter.Close()
	}
	if commandName == "exit" {
		if len(argsParts) > 0 && argsParts[0] == "0" {
			os.Exit(0)
		}
	} else if commandName == "echo" {
		fmt.Fprintln(outputWriter, argsString)
		return
	} else if commandName == "type" {
		if len(argsParts) == 0 {
			fmt.Fprintln(errWriter, "type takes two arguments but none were given")
			return
		}
		typeArg := strings.Join(argsParts, " ")
		if slices.Contains(shellBuiltIn, typeArg) {
			fmt.Fprintln(outputWriter, typeArg+typeFound)
			return
		}
		for i := range len(directories) {
			pathToExecutable, _ := checkForExecutable(directories[i], typeArg)
			if pathToExecutable != "" {
				fmt.Fprintln(outputWriter, typeArg+" is "+pathToExecutable)
				return
			}
		}
		fmt.Fprintln(errWriter, typeArg+": not found")
		return
	} else if commandName == "pwd" {
		if len(commandParts) > 1 {
			fmt.Fprintln(errWriter, "pwd takes no arguments but some were given")
			return
		}
		workingDir, err := os.Getwd()
		if err != nil {
			//fmt.Fprintln(errWriter, "Error running command: "+err.Error())
			return
		}
		fmt.Fprintln(outputWriter, workingDir)
		return
	} else if commandName == "cd" {
		if len(commandParts) != 2 {
			fmt.Fprintln(errWriter, "cd takes exactly one argument")
			return
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			//fmt.Fprintln(errWriter, "Error running command: "+err.Error())
			return
		}
		cdPath := argsString
		cleanedPath := path.Clean(strings.ReplaceAll(cdPath, "~", homeDir))
		err = os.Chdir(cleanedPath)
		if err != nil {
			if err.Error() == "chdir "+cdPath+": no such file or directory" {
				fmt.Fprintln(errWriter, "cd: "+cdPath+": No such file or directory")
				return
			}
			//fmt.Fprintln(errWriter, "Error running command: "+err.Error())
			return
		}
	} else {
		for i := range len(directories) {
			pathToExecutable, _ := checkForExecutable(directories[i], commandName)
			if pathToExecutable != "" {
				cmd := exec.Command(commandName, argsParts...)
				cmd.Stdin = os.Stdin
				cmd.Stdout = outputWriter
				cmd.Stderr = errWriter
				err := cmd.Run()
				if err != nil {
					//fmt.Fprintln(errWriter, "Error running command: "+err.Error())
				}
				return
			}
		}
		// command contains a trailing \n byte so we slice out that last bit
		fmt.Fprintln(errWriter, strings.Join(append([]string{commandName}, argsParts...), " ")+": command not found")

		return
	}
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
func parseCommandArgs(input string) (string, []string) {
	commandArgString := strings.TrimRight(input, "\r\n")
	args := []string{}
	var token strings.Builder
	escapeChar := false
	inDoubleQuotes := false
	inSingleQuotes := false
	for i, _ := range commandArgString {

		char := commandArgString[i]
		switch {
		case inSingleQuotes:
			if char == '\'' {
				inSingleQuotes = !inSingleQuotes
			} else {
				token.WriteByte(char)
			}
		case escapeChar:
			var escapeOptions []rune
			switch {
			case inDoubleQuotes:
				escapeOptions = escapeOptionsDoubleQuoted
			default:
				escapeOptions = escapeOptionUnquoted
			}
			if slices.Contains(escapeOptions, rune(char)) {
				token.WriteByte(char)
			} else {
				switch {
				case inDoubleQuotes:
					token.WriteByte('\\')
					token.WriteByte(char)
				case !inDoubleQuotes:
					token.WriteByte(char)
				}
			}
			escapeChar = false
		case char == '\\':
			// single quote already handled so in case of double or unquoted
			escapeChar = true
		case char == '"':
			inDoubleQuotes = !inDoubleQuotes
		case char == '\'':
			if !inDoubleQuotes {
				inSingleQuotes = !inSingleQuotes
			} else {
				token.WriteByte(char)
			}
		case char == ' ':
			if inDoubleQuotes {
				token.WriteByte(char)
			} else {
				if token.Len() > 0 {
					args = append(args, token.String())
					token.Reset()
				}
			}
		default:
			token.WriteByte(char)
		}
	}
	if token.Len() > 0 {
		args = append(args, token.String())
	}
	commandName := args[0]
	return commandName, args[1:]
}

func parseCommandName(input, commandName string) (string, int) {
	inDoubleQuotes := commandName[0] == '"'  // in double quotes
	inSingleQuotes := commandName[0] == '\'' // in single quotes

	commandName = ""
	escapedChar := false
	var i int = 0
	for k, char := range input[1:] {
		if inDoubleQuotes {
			if char == '"' && !escapedChar {
				// unescaped double quote if our name of command started with double quote then end
				i = k + 1
				break
			}
			if escapedChar {
				if slices.Contains(escapeOptionsDoubleQuoted, char) {
					commandName += string(char)
				} else {
					commandName += string('\\')
					commandName += string(char)
				}
				escapedChar = false
			} else {
				if char == '\\' {
					escapedChar = true
				} else {
					if !slices.Contains(escapeOptionsDoubleQuoted, char) {
						commandName += string(char)
					}
				}
			}
		} else if inSingleQuotes {
			if char == '\'' {
				// single quote encountered means end of command name
				i = k + 1
				break
			}
			commandName += string(char)
		}
	}
	return commandName, i
}

/*
	func parseOutputRedirect(input string) (string, string) {
		var i int = 0
		outputFilePath := ""
		errFilePath := ""
		foundRedirectSymbol := false
		foundErrRedirectSymbol := false
		inQuotes := false
		for {
			if i == len(input) {
				break
			}
			if !inQuotes {
				if input[i] == '>' {
					if input[i-1] == '2' {
						foundErrRedirectSymbol = true
						if foundRedirectSymbol {
							foundRedirectSymbol = false
						}
					} else {
						foundRedirectSymbol = true
						if foundErrRedirectSymbol {
							foundErrRedirectSymbol = false
						}
					}
				}
			}
			if foundRedirectSymbol {
				outputFilePath += string(input[i])
			}
			if foundErrRedirectSymbol {
				errFilePath += string(input[i])
			}
			i++
		}
		fmt.Println("FILE PATHS FOR REDIRECT")
		fmt.Println(outputFilePath)
		fmt.Println(errFilePath)
		return strings.Trim(outputFilePath, " >"), strings.Trim(errFilePath, " >")
	}
*/
func parseOutputRedirect(input string) (string, string) {
	stdOutRedirectPattern := `(?:^|[^2])>[\s]*"([^"]+)"|(?:^|[^2])>[\s]*'([^']+)'|(?:^|[^2])>[\s]*([^\s]+)`
	stdErrRedirectPattern := `2>[\s]*"([^"]+)"|2>[\s]*'([^']+)'|2>[\s]*(\S+)`

	stdOutReg := regexp.MustCompile(stdOutRedirectPattern)
	stdErrReg := regexp.MustCompile(stdErrRedirectPattern)

	stdOutMatch := stdOutReg.FindStringSubmatch(input)
	stdErrMatch := stdErrReg.FindStringSubmatch(input)
	stdOutRes := ""
	stdErrRes := ""
	if stdOutMatch != nil {
		stdOutRes = stdOutMatch[1] + stdOutMatch[2] + stdOutMatch[3]
	}
	if stdErrMatch != nil {
		stdErrRes = stdErrMatch[1] + stdErrMatch[2] + stdErrMatch[3]
	}
	return stdOutRes, stdErrRes

}

func removeRedirection(input string) string {
	stdOutRedirectPattern := `(?:^|[^2])>[\s]*"([^"]+)"|(?:^|[^2])>[\s]*'([^']+)'|(?:^|[^2])>[\s]*([^\s]+)`
	stdErrRedirectPattern := `2>[\s]*"([^"]+)"|2>[\s]*'([^']+)'|2>[\s]*(\S+)`

	stdOutReg := regexp.MustCompile(stdOutRedirectPattern)
	stdErrReg := regexp.MustCompile(stdErrRedirectPattern)

	res := stdOutReg.ReplaceAllString(input, "")
	res = stdErrReg.ReplaceAllString(res, "")
	return res
}
