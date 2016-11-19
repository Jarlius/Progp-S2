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

type Dot struct {}
type Cit struct {}

type Command struct {
	name string
	arg string
	list []Command
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
				if dot == "." {
					tokens <- Dot{}
				} else if dot == `"` {
					tokens <- Cit{}
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
	var cit_count int
	var next Command
	var prev interface{}
	for token := range tokens {
		switch prev := prev.(type) {
		case Word:
			switch prev.word.(type) {
	// FORW|BACK|LEFT|RIGHT|REP -> INT
			case IntWord:
				if arg,b := token.(Int); b {
					next.arg = arg.val
				}
	// COLOR -> COL
			case ColWord: 
				if arg,b := token.(Color); b {
					next.arg = arg.val
				}
	// DOWN|UP -> DOT
			case DotWord: 
				next = dotting(next,token,commands,wg,cit_count)
			}
		case Int:
	// INT -> CIT|CMD
			if next.name == "REP" && cit_count == 0 { 
				// TODO: nytt rep = flytta målet för replista till next.list
				if _,b := token.(Cit); b {
					cit_count++
				} else {
					next = ins_cmd(next,token)
				}
	// INT -> DOT
			} else { 
				next = dotting(next,token,commands,wg,cit_count)
			}
	// COL -> DOT
		case Color: 
			next = dotting(next,token,commands,wg,cit_count)
		case Dot:
	// DOT -> CIT
			if _,b := token.(Cit); b && (cit_count != 0) { 
				cit_count--
				// TODO: multiplicera djupaste listan
				// TODO: sätt nytt mål ett steg högre
	// DOT -> CMD
			} else { 
				next = ins_cmd(next,token)
			}
		case Cit:
	// CIT -> CIT
			if _,b := token.(Cit); b && (cit_count != 0) { 
				cit_count--
				// TODO: multiplicera djupaste listan
				// TODO: sätt nytt mål ett steg högre
	// CIT -> CMD
			} else { 
				next = ins_cmd(next,token)
			}
		default:
			next = ins_cmd(next,token)
		}
		prev = token
	}
}

func ins_cmd(target Command, token interface{}) Command {
	if cmd,b := token.(Word); b {
		target.name = reflect.ValueOf(cmd.word).Field(0).String()
	}
	return target
}

func dotting(target Command, token interface{}, cmds chan<- Command, wg *sync.WaitGroup, cits int) Command {
	if _,b := token.(Dot); b {
		if cits == 0 {
			// TODO: om next.name är REP så skicka dess lista r gånger till cmds
			wg.Add(1)
			cmds <- target
			target = Command{}
		} else {
			target.list = append(target.list,Command{target.name,target.arg,[]Command{}})
		}
	}
	return target
}

func Executor(commands <-chan Command, wg *sync.WaitGroup, output chan<- string) {
	var answer string
	for command := range commands {
		answer += "{" + command.name + " " + command.arg + "} "
		wg.Done()
	}
	output <- answer
}

