package main

import (
	"bufio"
	"os"
	"sync"
	"strings"
	"strconv"
	"regexp"
	"math"
	"fmt"
)

type Token struct {row int;tok interface{}}
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
	nocit bool
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	input := make(chan string)
	tokens := make(chan Token)
	commands := make(chan Command)
	output := make(chan string)
	wait_rows := new(sync.WaitGroup)
	wait_toks := new(sync.WaitGroup)
	wait_exec := new(sync.WaitGroup)
	error_row := make(chan int)
	
	go Analyser(input,tokens,wait_rows,wait_toks,error_row)
	go Parser(tokens,commands,wait_toks,wait_exec,error_row)
	go Executor(commands,wait_exec,output)
	
	wait_rows.Add(1)
	go func() {
		for scanner.Scan() {
			wait_rows.Add(1)
			input <- scanner.Text()
		}
		wait_rows.Done()
	}()

	go func() {
		wait_rows.Wait()
		wait_toks.Wait()
		wait_exec.Wait()
		error_row <- 0
	}()

	if erow := <-error_row; erow != 0 {
		fmt.Printf("Syntaxfel på rad %d\n", erow)
	} else {
		close(tokens)
		if last := <-error_row; last != 0 {
			fmt.Printf("Syntaxfel på rad %d\n", last)
			return
		}
		close(commands)
		fmt.Println(<-output)
	}
}

func Analyser(input <-chan string, tokens chan<- Token, wait_rows *sync.WaitGroup, wait_toks *sync.WaitGroup, erow chan<- int) {
	spacgex,_ := regexp.Compile(`^\s*([^\s]+[\s\.\%]|[\."])$`) 
	iwordex,_ := regexp.Compile(`^(FORW|BACK|LEFT|RIGHT|REP)$`)
	cwordex,_ := regexp.Compile(`^COLOR$`)
	dwordex,_ := regexp.Compile(`^(DOWN|UP)$`)
	colorex,_ := regexp.Compile(`^\#[A-Z\d]{6}$`)
	integex,_ := regexp.Compile(`^[1-9]\d*$`)
	nullgex,_ := regexp.Compile(`^\s*\%$`)
	row := 1
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
					tokens <- Token{row,Word{IntWord{trim}}}
				case cwordex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Token{row,Word{ColWord{trim}}}
				case dwordex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Token{row,Word{DotWord{trim}}}
				case integex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Token{row,Int{trim}}
				case colorex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Token{row,Color{trim}}
				case trim == "":
				default:
					erow <- row
				}
				if dot == "." {
					wait_toks.Add(1)
					tokens <- Token{row,Dot{}}
				} else if dot == `"` {
					wait_toks.Add(1)
					tokens <- Token{row,Cit{}}
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
		row++
		wait_rows.Done()
	}
}

func Parser(tokens <-chan Token, commands chan<- Command, wait_toks *sync.WaitGroup, wait_exec *sync.WaitGroup, erow chan<- int) {
	var last_row int
	var cit_count int
	var next Command
	var cur *Command = &next
	var prev interface{} = Dot{}
	for tokenstruct := range tokens {
		token := tokenstruct.tok
		last_row = tokenstruct.row
		switch prev := prev.(type) {
		case Word:
			switch prev.word.(type) {
	// FORW|BACK|LEFT|RIGHT|REP -> INT
			case IntWord:
				if arg,b := token.(Int); b {
					next.arg = arg.val
				} else {
					erow <- tokenstruct.row
				}
	// COLOR -> COL
			case ColWord:
				if arg,b := token.(Color); b {
					next.arg = arg.val
				} else {
					erow <- tokenstruct.row
				}
	// DOWN|UP -> DOT
			case DotWord: 
				cur = dotting(&next,cur,tokenstruct,commands,wait_exec,erow)
			}
		case Int:
	// INT -> CIT|CMD
			if next.name == "REP" { 
				// Sätter cur ett steg lägre för ny lista
				(*cur).list = append((*cur).list,Command{})
				cur = &(*cur).list[len((*cur).list)-1]
				if _,b := token.(Cit); b {
					*cur = Command{next.name,next.arg,[]Command{},false}
					cit_count++
				} else {
					*cur = Command{next.name,next.arg,[]Command{},true}
					next = InsertWord(next,tokenstruct,erow)
				}
	// INT -> DOT
			} else { 
				cur = dotting(&next,cur,tokenstruct,commands,wait_exec,erow)
			}
	// COL -> DOT
		case Color: 
			cur = dotting(&next,cur,tokenstruct,commands,wait_exec,erow)
		case Dot:
	// DOT -> CIT
			if _,b := token.(Cit); b && (cit_count != 0) { 
				cit_count--
				// sätt nytt mål ett steg högre
				cur = Backtrack(&next,cur)
				cur = exit(&next,cur,commands,wait_exec)
	// DOT -> CMD
			} else { 
				next = InsertWord(next,tokenstruct,erow)
			}
		case Cit:
	// CIT -> CIT
			if _,b := token.(Cit); b && (cit_count != 0) && (next.name != "REP") { 
				cit_count--
				// sätt nytt mål ett steg högre
				cur = Backtrack(&next,cur)
				cur = exit(&next,cur,commands,wait_exec)
	// CIT -> CMD
			} else { 
				next = InsertWord(next,tokenstruct,erow)
			}
		}
		prev = token
		wait_toks.Done()
	}
	_,dot := prev.(Dot)
	_,cit := prev.(Cit)
	if cit_count != 0 || (!dot && !cit) {
		erow <- last_row
	} else {
		erow <- 0
	}
}

func InsertWord(target Command, tokenstruct Token, erow chan<- int) Command {
	token := tokenstruct.tok
	if cmd,b := token.(Word); b {
		switch word := cmd.word.(type) {
		case IntWord:
			target.name = word.val
		case ColWord:
			target.name = word.val
		case DotWord:
			target.name = word.val
		}
	} else {
		erow <- tokenstruct.row
	}
	return target
}

func dotting(source *Command, target *Command, tokenstruct Token, cmds chan<- Command, wg *sync.WaitGroup, erow chan<- int) *Command {
	token := tokenstruct.tok
	if _,b := token.(Dot); b {
		if len((*source).list) == 0 {
			wg.Add(1)
			cmds <- *source
			*source = Command{}
			target = source // onödigt?
		} else {
			(*target).list = append((*target).list,Command{source.name,source.arg,[]Command{},false})
			target = exit(source,target,cmds,wg)
		}
	} else {
		erow <- tokenstruct.row
	}
	return target
}

func exit(source *Command,target *Command,cmds chan<- Command, wg *sync.WaitGroup) *Command {
	for (*target).nocit {
		target = Backtrack(source,target)
	}
	if source == target {
		Repeat((*source).list,cmds,wg)
		*source = Command{}
	}
	return target
}

func Backtrack(first *Command,current *Command) *Command {
	if len((*first).list) == 0 {
		return first
	}
	edge := &(*first).list[len(first.list)-1]
	if edge == current {
		return first
	}
	return Backtrack(edge,current)
}

func Repeat(list []Command,cmds chan<- Command, wg *sync.WaitGroup) {
	for _,cmd := range list {
		if cmd.name == "REP" {
			reps,_ := strconv.Atoi(cmd.arg)
			for i := 0; i < reps; i++ {
				Repeat(cmd.list,cmds,wg)
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
