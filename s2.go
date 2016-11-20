package main

import (
	"bufio"
	"os"
	"sync"
	"strings"
	"strconv"
	"math"
	"regexp"
	"reflect"
	"fmt"
)

type Word struct {word interface{}}
type IntWord struct {val string}
type ColWord struct {val string}
type DotWord struct {val string}

type Color struct {val string}
type Int struct {val string}

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
	badorder := make(chan bool)
	commands := make(chan Command)
	output := make(chan string)
	wait_rows := new(sync.WaitGroup)
	wait_toks := new(sync.WaitGroup)
	wait_exec := new(sync.WaitGroup)

	done := make(chan bool)
	go func() {
		wait_rows.Wait()
		wait_toks.Wait()
		wait_exec.Wait()
		done <- true
	}()
	
	go Analyser(input,tokens,badsyntax,wait_rows,wait_toks)
	go Parser(tokens,commands,badorder,wait_toks,wait_exec)
	go Executor(commands,wait_exec,output)
	
	for i := 1; scanner.Scan(); i++ {
		input <- scanner.Text()
		wait_rows.Add(1)
		if <-badsyntax {
			fmt.Printf("Syntaxfel på rad %d\n", i)
			return
		}
	}

	
	if <-done {
		close(commands)
		fmt.Println(<-output)
	}
/*	wait_rows.Wait()
	wait_toks.Wait()
	wait_exec.Wait()
	close(commands)
	fmt.Println(<-output)
*/}

func Analyser(input <-chan string, tokens chan<- interface{}, bad chan<- bool, wait_rows *sync.WaitGroup, wait_toks *sync.WaitGroup) {
	spacgex,_ := regexp.Compile(`^\s*([^\s]+[\s\.\%]|[\."])$`) 
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
			comment := false
			if spacgex.MatchString(word) {
				dot := ""
				if last := string(r); last == "." || last == `"` || last == "%" {
					dot = last
					word = strings.TrimSuffix(word,last)
				}
				trim := strings.TrimSpace(word)
				switch {
				case iwordex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Word{IntWord{trim}}
				case cwordex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Word{ColWord{trim}}
				case dwordex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Word{DotWord{trim}}
				case integex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Int{trim}
				case colorex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Color{trim}
				case trim == "":
				default:
					bad <- true
				}
				if dot == "." {
					wait_toks.Add(1)
					tokens <- Dot{}
				} else if dot == `"` {
					wait_toks.Add(1)
					tokens <- Cit{}
				} else if dot == "%" {
					comment = true
				}
				word = ""
			} else if nullgex.MatchString(word) {
				break
			}
			if comment {
				break
			}
		}
		bad <- false
		wait_rows.Done()
	}
}

func Parser(tokens <-chan interface{}, commands chan<- Command, bad chan<- bool, wait_toks *sync.WaitGroup, wait_exec *sync.WaitGroup) {
	var cit_count int
	var next Command
	var cur *Command = &next
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
				// TODO: här kan det bli syntaxfel
	// COLOR -> COL
			case ColWord:
				if arg,b := token.(Color); b {
					next.arg = arg.val
				}
				// TODO: här kan det bli syntaxfel
	// DOWN|UP -> DOT
			case DotWord: 
				next = dotting(next,cur,token,commands,wait_exec,cit_count)
			}
		case Int:
	// INT -> CIT|CMD
			if next.name == "REP" { 
				// Sätter cur ett steg lägre för ny lista
				(*cur).list = append((*cur).list,Command{})
				cur = &(*cur).list[len((*cur).list)-1]
				*cur = Command{next.name,next.arg,[]Command{}}
				if _,b := token.(Cit); b {
					cit_count++
				} else {
					next = ins_cmd(next,token)
				}
	// INT -> DOT
			} else { 
				next = dotting(next,cur,token,commands,wait_exec,cit_count)
			}
	// COL -> DOT
		case Color: 
			next = dotting(next,cur,token,commands,wait_exec,cit_count)
		case Dot:
	// DOT -> CIT
			if _,b := token.(Cit); b && (cit_count != 0) { 
				cit_count--
				// sätt nytt mål ett steg högre
				cur = backtrack(&next,cur)
				if cit_count == 0 {
					repeat(next.list,commands,wait_exec)
					next = Command{}
				}
	// DOT -> CMD
			} else { 
				next = ins_cmd(next,token)
			}
		case Cit:
	// CIT -> CIT
			if _,b := token.(Cit); b && (cit_count != 0) { 
				cit_count--
				// sätt nytt mål ett steg högre
				cur = backtrack(&next,cur)
				if cit_count == 0 {
					repeat(next.list,commands,wait_exec)
					next = Command{}
				}
	// CIT -> CMD
			} else { 
				next = ins_cmd(next,token)
			}
		default:
			next = ins_cmd(next,token)
		}
		prev = token
		wait_toks.Done()
	}
}

func ins_cmd(target Command, token interface{}) Command {
	if cmd,b := token.(Word); b {
		target.name = reflect.ValueOf(cmd.word).Field(0).String()
	}
	// TODO: här kan det bli syntaxfel
	return target
}

func dotting(source Command, target *Command, token interface{}, cmds chan<- Command, wg *sync.WaitGroup, cits int) Command {
	if _,b := token.(Dot); b {
		if cits == 0 {
			// om replistan inte är tom, skicka den till repeat
			if len(source.list) != 0 {
				(*target).list = append((*target).list,Command{source.name,source.arg,[]Command{}})
				repeat(source.list,cmds,wg)
			} else {
				wg.Add(1)
				cmds <- source
			}
			source = Command{}
			target = &source
		} else {
			(*target).list = append((*target).list,Command{source.name,source.arg,[]Command{}})
		}
	}
	// TODO: här kan det bli syntaxfel
	return source
}

func backtrack(first *Command,current *Command) *Command {
	if len((*first).list) == 0 {
		return first
	}
	edge := &(*first).list[len(first.list)-1]
	if edge == current {
		return first
	}
	return backtrack(edge,current)
}

func repeat(list []Command,cmds chan<- Command, wg *sync.WaitGroup) {
	for _,cmd := range list {
		if cmd.name == "REP" {
			reps,_ := strconv.Atoi(cmd.arg)
			for i := 0; i < reps; i++ {
				repeat(cmd.list,cmds,wg)
			}
		} else {
			wg.Add(1)
			cmds <- cmd
		}
	}
}

func Executor(commands <-chan Command, wg *sync.WaitGroup, output chan<- string) {
	var answer string
	down:= false	
	color:= "#0000ff"
	rotation:= 0 //(int) degrees
	x1:= 0.0 //(float64)
	y1:= 0.0 //(float64)
	for command := range commands {
		x2:=x1
		y2:=y1
		switch command.name {
			case "DOWN":
					down = true
			case "UP":
					down = false
			case "FORW": 
				y2+=math.Sin((float64(rotation)*math.Pi/180))*StringToFloat(command.arg)
				x2+=math.Cos((float64(rotation)*math.Pi/180))*StringToFloat(command.arg)

				if down {
					answer += color + " " + FloatToString(x1) + " " + FloatToString(y1) + " " + FloatToString(x2)+ " " + FloatToString(y2) + "\n"
				}											
			case "BACK":
				y2-=math.Sin((float64(rotation)*math.Pi/180))*StringToFloat(command.arg)
				x2-=math.Cos((float64(rotation)*math.Pi/180))*StringToFloat(command.arg)
				//går att bli av med i extern funktion men blir mer kod
				if down {
					answer += color + " " + FloatToString(x1) + " " + FloatToString(y1) + " " + FloatToString(x2)+ " " + FloatToString(y2) + "\n"
				}
			case "LEFT":
				rotation+=StringToInt(command.arg)
				
			case "RIGHT":
				rotation-=StringToInt(command.arg)
			case "COLOR": 
				color = command.arg
			//case "REP": hanteras inte här 
			default: 
				//TODO error message
		}
		//ready for next
		x1=x2
		y1=y2	
		//done
		wg.Done() 
	}
	output <- answer
}

func FloatToString(convert float64) string {
    // to convert a float number to a string
    return strconv.FormatFloat(convert, 'f', 6, 64)
}

func StringToFloat(convert string) float64 {
     if n, err:= strconv.ParseFloat(convert, 64); err == nil {
            return n
     } else {
     	//TODO error message
     	return 0.0

     }
}

func StringToInt(convert string) int {
	if r, err:= strconv.Atoi(convert);err==nil{
		return r
	} else {
		//TODO Error message
		return 0	
	}
}
