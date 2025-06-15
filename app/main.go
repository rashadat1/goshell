package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/chzyer/readline"
)

const typeFound string = " is a shell builtin"

var shellBuiltIn []string = []string{"echo", "exit", "type", "pwd", "cd"}
var escapeOptionsDoubleQuoted []rune = []rune{'\\', '$', '"', ' '}
var escapeOptionUnquoted []rune = []rune{'\\', '$', '"', ' ', '\''}

// the AutoCompleter interface requires one method
// Do(line []rune, pos int) (newLine [][]rune, length int)
// AutoComplete in the readline.Config struct is of type AutoCompleter
// so we need to give our TabAutoCompleter a Do method with this
// signature, instantiate the TabAutoCompleter and pass it as the autocompleter
type TabAutoCompleter struct {
	Commands  []string
	Path      string
	TabCount  int
	LastInput string
}

func (tac *TabAutoCompleter) Do(line []rune, pos int) ([][]rune, int) {
	input := string(line[:pos])

	autoCompleteResults := make([][]rune, 0)
	executableResults := getExecutables(tac.Path, input)
	for _, cmd := range tac.Commands {
		if strings.HasPrefix(cmd, input) {
			autoCompleteResults = append(autoCompleteResults, []rune(cmd[pos:]+" "))
		}
	}
	for _, cmdExec := range executableResults {
		autoCompleteResults = append(autoCompleteResults, []rune(cmdExec[pos:]+" "))
	}
	if len(autoCompleteResults) == 0 {
		fmt.Fprint(os.Stdout, "\x07")
		return nil, pos
	}
	sort.Slice(autoCompleteResults, func(i, j int) bool {
		return string(autoCompleteResults[i]) < string(autoCompleteResults[j])
	})
	if len(executableResults) == 1 {
		return [][]rune{[]rune(executableResults[0][pos:] + " ")}, pos
	}
	if len(executableResults) == 0 && len(autoCompleteResults) >= 1 {
		return autoCompleteResults, pos
	}
	if len(executableResults) > 1 {
		autoCompleteStrings := make([]string, 0)
		shortestMatch := findShortestString(autoCompleteResults)
		hasSharedPrefix := haveSharedPrefix(shortestMatch, autoCompleteResults)
		if hasSharedPrefix {
			return [][]rune{[]rune(shortestMatch)}, pos
		} else {
			if tac.TabCount == 0 {
				fmt.Fprint(os.Stdout, "\a")
				tac.TabCount++
				tac.LastInput = input
				return nil, pos
			} else {
				for _, match := range executableResults {
					autoCompleteStrings = append(autoCompleteStrings, match)
				}
				sort.Slice(autoCompleteStrings, func(i, j int) bool {
					return string(autoCompleteStrings[i]) < string(autoCompleteStrings[j])
				})
				fmt.Println()
				fmt.Println(strings.Join(autoCompleteStrings, "  "))
				fmt.Printf("$ %s", input)
				tac.TabCount++
			}
		}

	}
	return nil, pos
}
func findShortestString(autoCompleteResults [][]rune) string {
	shortestLength := 100000
	shortestCandidate := ""
	for _, result := range autoCompleteResults {
		if len(result) < shortestLength {
			shortestLength = len(result)
			shortestCandidate = string(result)
		}
	}
	return strings.Trim(shortestCandidate, " ")
}
func haveSharedPrefix(shortestMatch string, autoCompleteResults [][]rune) bool {
	for _, runeSliceRes := range autoCompleteResults {
		stringSliceRes := string(runeSliceRes)
		if !strings.HasPrefix(stringSliceRes, shortestMatch) {
			return false
		}
	}
	return true
}
func main() {
	PATH := os.Getenv("PATH")
	completer := &TabAutoCompleter{
		Commands: shellBuiltIn,
		Path:     PATH,
		TabCount: 0,
	}
	l, err := readline.NewEx(&readline.Config{
		Prompt:       "$ ",
		AutoComplete: completer,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		_, err := fmt.Fprint(os.Stdout, "$ ")
		if err != nil {
			log.Println("Error writing shell intialization bytes to stdout: " + err.Error())
		}
		command, err := l.Readline()
		if err != nil {
			log.Println("Error reading string from standard in " + err.Error())
		}
		commandProcessor(command, PATH)
		completer.TabCount = 0
		completer.LastInput = ""
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
	outputAppendFilePath := ""
	errFileAppendFilePath := ""
	outputWriter := os.Stdout
	errWriter := os.Stdout

	// create an argParts without the redirection symbol

	outputFilePath, errFilePath, outputAppendFilePath, errFileAppendFilePath = parseOutputRedirect(input)

	// remove redirection so this is not interpreted as a command argument
	removedRedirect := removeRedirection(input)
	cmdParsed, argsParts := parseCommandArgs(removedRedirect)

	commandName = cmdParsed
	argsString := strings.Join(argsParts, " ")
	var err error
	if outputFilePath != "" {
		os.MkdirAll(filepath.Dir(outputFilePath), 0755)
		outputWriter, err = os.OpenFile(outputFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		defer outputWriter.Close()
	}
	if errFilePath != "" {
		os.MkdirAll(filepath.Dir(errFilePath), 0755)
		errWriter, err = os.OpenFile(errFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		defer errWriter.Close()
	}
	if outputAppendFilePath != "" {
		os.MkdirAll(filepath.Dir(outputAppendFilePath), 0755)
		outputWriter, err = os.OpenFile(outputAppendFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		defer outputWriter.Close()
	}
	if errFileAppendFilePath != "" {
		os.MkdirAll(filepath.Dir(errFileAppendFilePath), 0755)
		errWriter, err = os.OpenFile(errFileAppendFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		defer errWriter.Close()
	}
	if err != nil {
		fmt.Println("Error creating out/err writer: " + err.Error())
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
func checkForExecutableSuffix(path, input string) ([]string, error) {
	c, err := os.ReadDir(path)
	res := make([]string, 0)
	if err != nil {
		return nil, err
	}
	for _, entry := range c {
		if strings.HasPrefix(entry.Name(), input) {
			res = append(res, entry.Name())
		}
	}
	return res, nil
}
func getExecutables(PATH string, input string) []string {
	directories := strings.Split(PATH, ":")
	res := make([]string, 0)
	for i := range len(directories) {
		pathsToExecutables, _ := checkForExecutableSuffix(directories[i], input)
		res = append(res, pathsToExecutables...)
	}
	return res
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
func parseOutputRedirect(input string) (string, string, string, string) {
	stdOutRedirectPattern := `(?:^|\s)1?>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`
	stdOutAppendPattern := `(?:^|\s)1?>>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`
	stdErrRedirectPattern := `(?:^|\s)2{1}>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`
	stdErrAppendPattern := `(?:^|\s)2{1}>>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`

	stdOutReg := regexp.MustCompile(stdOutRedirectPattern)
	stdErrReg := regexp.MustCompile(stdErrRedirectPattern)

	stdOutAppendReg := regexp.MustCompile(stdOutAppendPattern)
	stdErrAppendReg := regexp.MustCompile(stdErrAppendPattern)

	stdOutMatch := stdOutReg.FindStringSubmatch(input)
	stdErrMatch := stdErrReg.FindStringSubmatch(input)

	stdOutAppendMatch := stdOutAppendReg.FindStringSubmatch(input)
	stdErrAppendMatch := stdErrAppendReg.FindStringSubmatch(input)

	stdOutRes := ""
	stdErrRes := ""
	stdOutAppendRes := ""
	stdErrAppendRes := ""
	if stdOutMatch != nil {
		stdOutRes = stdOutMatch[1] + stdOutMatch[2] + stdOutMatch[3]
	}
	if stdErrMatch != nil {
		stdErrRes = stdErrMatch[1] + stdErrMatch[2] + stdErrMatch[3]
	}
	if stdOutAppendMatch != nil {
		stdOutAppendRes = stdOutAppendMatch[1] + stdOutAppendMatch[2] + stdOutAppendMatch[3]
	}
	if stdErrAppendMatch != nil {
		stdErrAppendRes = stdErrAppendMatch[1] + stdErrAppendMatch[2] + stdErrAppendMatch[3]
	}
	return stdOutRes, stdErrRes, stdOutAppendRes, stdErrAppendRes

}

func removeRedirection(input string) string {
	stdOutRedirectPattern := `(?:^|\s)1?>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`
	stdOutAppendPattern := `(?:^|\s)1?>>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`
	stdErrRedirectPattern := `(?:^|\s)2{1}>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`
	stdErrAppendPattern := `(?:^|\s)2{1}>>(?:\s*"([^"]+)"|\s*'([^']+)'|\s*([^\s>]+))`

	stdOutReg := regexp.MustCompile(stdOutRedirectPattern)
	stdErrReg := regexp.MustCompile(stdErrRedirectPattern)
	stdOutAppendReg := regexp.MustCompile(stdOutAppendPattern)
	stdErrAppendReg := regexp.MustCompile(stdErrAppendPattern)

	res := stdOutReg.ReplaceAllString(input, "")
	res = stdErrReg.ReplaceAllString(res, "")
	res = stdOutAppendReg.ReplaceAllString(res, "")
	res = stdErrAppendReg.ReplaceAllString(res, "")
	return res
}
