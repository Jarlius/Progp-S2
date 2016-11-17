package main

import (
	"bufio"
	"os"
	"sync"
	"strings"
	"strconv"
	"regexp"
	"fmt"
)

type Color struct {
	val string
}

type Command struct {
	name string
	arg string
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	tokens := make(chan interface{})
	commands := make(chan Command)
	output := make(chan string)
	wg := new(sync.WaitGroup)
	
	go Parser(tokens,commands)
	go Executor(commands,wg,output)
	for i := 1; scanner.Scan(); i++ {
		if !Analyser(scanner.Text(),tokens,wg) { // kanske göra kanal som skickar till analyser
			fmt.Printf("Syntaxfel på rad %d\n", i)
			return
		}
	}
	
	wg.Wait()
	close(commands)
	fmt.Println(<-output)
}

func Analyser(input string, tokens chan<- interface{}, wg *sync.WaitGroup) bool {
	words := strings.ToUpper(input + " ")
	word := " "
	spacgex,_ := regexp.Compile(`^\s*([^\s]+[\s\.]|[\."])$`) 
	wordgex,_ := regexp.Compile(`^(FORW|BACK|LEFT|RIGHT|DOWN|UP|COLOR|REP)$`)
	colorex,_ := regexp.Compile(`^\#[A-Z\d]{6}$`)
	integex,_ := regexp.Compile(`^\d+$`)
	nullgex,_ := regexp.Compile(`^\s*\%$`)
	for _,r := range words {
		word += string(r)
		if spacgex.MatchString(word) {
			dot := ""
			if last := string(word[len(word)-1]); last == "." || last == `"` {
				dot = last
				word = strings.TrimSuffix(word,last)
			}
			trim := strings.TrimSpace(word)
			switch {
			case wordgex.MatchString(trim):
				wg.Add(1)
				tokens <- trim
			case colorex.MatchString(trim):
				wg.Add(1)
				tokens <- Color{trim}
			case integex.MatchString(trim):
				wg.Add(1)
				n,_ := strconv.Atoi(trim)
				tokens <- n
			case trim == "":
			default:
				return false
			}
			if dot != "" {
				wg.Add(1)
				tokens <- dot
			}
			word = ""
		} else if nullgex.MatchString(word) {
			return true
		}
	}
	return true
}

func Parser(tokens <-chan interface{}, commands chan<- Command) {
	for token := range tokens {
		//TODO
		switch token := token.(type) {
		case string:
			commands <- Command{token,""}
		case int:
			commands <- Command{"int",""}
		case Color:
			commands <- Command{"color",""}
		}
	}
}

func Executor(commands <-chan Command, wg *sync.WaitGroup, output chan<- string) {
	var answer string
	for command := range commands {
		//TODO
		answer += command.name + " "
		wg.Done()
	}
	output <- answer
}

