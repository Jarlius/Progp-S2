package main

import (
	"bufio"
	"os"
	"sync"
	"strings"
	"regexp"
	"reflect"
	"fmt"
)

type Word struct {
	word interface{}	
}

type IntWord struct {
	val string
}

type ColWord struct {
	val string
}

type DotWord struct {
	val string
}

type Color struct {
	val string
}

type Int struct {
	val string
}

type Dot struct {
	val string
}

type Command struct {
	name string
	arg string
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	input := make(chan string)
	tokens := make(chan interface{})
	badsyntax := make(chan bool)
	commands := make(chan Command)
	output := make(chan string)
	wg := new(sync.WaitGroup)

	go Analyser(input,tokens,badsyntax)
	go Parser(tokens,commands,wg)
	go Executor(commands,wg,output)
	
	for i := 1; scanner.Scan(); i++ {
		input <- scanner.Text()
		if <-badsyntax {
			fmt.Printf("Syntaxfel på rad %d\n", i)
			return
		}
	}

	wg.Wait()
	close(commands)
	fmt.Println(<-output)
}

func Analyser(input <-chan string, tokens chan<- interface{}, bad chan<- bool) {
	spacgex,_ := regexp.Compile(`^\s*([^\s]+[\s\.]|[\."])$`) 
	iwordex,_ := regexp.Compile(`^(FORW|BACK|LEFT|RIGHT|REP)$`)
	cwordex,_ := regexp.Compile(`^COLOR$`)
	dwordex,_ := regexp.Compile(`^(DOWN|UP)$`)
	colorex,_ := regexp.Compile(`^\#[A-Z\d]{6}$`)
	integex,_ := regexp.Compile(`^\d+$`)
	nullgex,_ := regexp.Compile(`^\s*\%$`)
	for s := range input {
		words := strings.ToUpper(s + " ")
		word := " "
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
				case iwordex.MatchString(trim):
					tokens <- Word{IntWord{trim}}
				case cwordex.MatchString(trim):
					tokens <- Word{ColWord{trim}}
				case dwordex.MatchString(trim):
					tokens <- Word{DotWord{trim}}
				case integex.MatchString(trim):
					tokens <- Int{trim}
				case colorex.MatchString(trim):
					tokens <- Color{trim}
				case trim == "":
				default:
					bad <- true
				}
				if dot != "" {
					tokens <- Dot{dot}
				}
				word = ""
			} else if nullgex.MatchString(word) {
				break
			}
		}
		bad <- false
	}
}

func Parser(tokens <-chan interface{}, commands chan<- Command, wg *sync.WaitGroup) {
	var next Command
	var prev interface{}
	for token := range tokens {
		switch prev := prev.(type) {
		case Word:
			switch prev.word.(type) {
			case IntWord:
				if arg,b := token.(Int); b {
					next.arg = arg.val
				}
			case ColWord:
				if arg,b := token.(Color); b {
					next.arg = arg.val
				}
			case DotWord:
				if _,b := token.(Dot); b {
					wg.Add(1)
					commands <- next
					next = Command{}
				}
			}
		case Int:
			if _,b := token.(Dot); b {
				wg.Add(1)
				commands <- next
				next = Command{}
			}
		case Color:
			if _,b := token.(Dot); b {
				wg.Add(1)
				commands <- next
				next = Command{}
			}
		case Dot:
			if cmd,b := token.(Word); b {
				next.name = reflect.ValueOf(cmd.word).Field(0).String()
			}
		default:
			if cmd,b := token.(Word); b {
				next.name = reflect.ValueOf(cmd.word).Field(0).String()
			}
		}
		prev = token
	}
}

func Executor(commands <-chan Command, wg *sync.WaitGroup, output chan<- string) {
	var answer string
	for command := range commands {
		answer += command.name + " " + command.arg
		wg.Done()
	}
	output <- answer
}

