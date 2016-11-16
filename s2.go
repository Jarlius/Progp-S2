package main

import (
	"bufio"
	"os"
	"sync"
	"strings"
	"regexp"
	"fmt"
)

type Command struct {
	name string
	arg string
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	tokens := make(chan string)
	commands := make(chan Command)
	output := make(chan string)
	wg := new(sync.WaitGroup)
	
	go Parser(tokens,commands)
	go Executor(commands,wg,output)
	for i := 1; scanner.Scan(); i++ {
		if !Analyser(scanner.Text(),tokens,wg) {
			fmt.Printf("Syntaxfel på rad %d\n", i)
			return
		}
	}
	
	wg.Wait()
	close(commands)
	fmt.Println(<-output)
}

func Analyser(input string, tokens chan<- string, wg *sync.WaitGroup) bool {
	words := strings.ToLower(input)
	current := " "
	regex,_ := regexp.Compile(`^(` +
		`(\s*(forw|back|left|right|down|up|color|rep|\.|"))|` +
		`(\s+(\d+|\#[a-z\d]{6})[\s\.])` +
	")$")
	comex,_ := regexp.Compile(`^\s*\%$`)
	for _,r := range words {
		current += (string(r))
		if regex.MatchString(current) {
			trimmed := strings.TrimSpace(current)
			wg.Add(1)
			if string(trimmed[len(trimmed)-1]) == "." {
				if trimmed != "." {
					wg.Add(1)
					tokens <- strings.TrimSuffix(trimmed,".")
				}
				tokens <- "."
			} else {
				tokens <- trimmed
			}
			current = ""
		} else if comex.MatchString(current) {
			return true
		}
	}
	if 	strings.TrimSpace(current) != "" {
		return false
	}
	return true
}

func Parser(tokens <-chan string, commands chan<- Command) {
	for token := range tokens {
		//TODO
		commands <- Command{token,""}
	}
}

func Executor(commands <-chan Command, wg *sync.WaitGroup, output chan<- string) {
	var answer string
	for command := range commands {
		//TODO
		answer += command.name
		wg.Done()
	}
	output <- answer
}

