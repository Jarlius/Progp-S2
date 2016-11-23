// Laboration S2 av Jarl Silvén och Simon Hellberg
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
	back *Command
	nocit bool
}

// Mainfunktion, skapar kanaler, trådar och skriver ut slutresultat
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	input := make(chan string)
	tokens := make(chan Token)
	reps := make(chan []Command)
	commands := make(chan Command)
	output := make(chan string)
	wait_rows := new(sync.WaitGroup)
	wait_toks := new(sync.WaitGroup)
	wait_send := new(sync.WaitGroup)
	wait_exec := new(sync.WaitGroup)
	error_row := make(chan int)
	
	go Analyser(input,tokens,wait_rows,wait_toks,error_row)
	go Parser(tokens,reps,wait_toks,wait_send,error_row)
	go Sender(reps,commands,wait_send,wait_exec)
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
		wait_send.Wait()
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

// Tråd för Lexikal Analys, tar emot en rad från Scanner i taget och skickar
// dess beståndsdelar som tokens. Om tecken inte skapar ett token skickas 
// felmeddelande till mainfuntion.
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

// Tråd för Parsning. Tar emot tokens från Analysen i ordning och kollar att
// kollar att de är semantiskt korrekta, dvs att tokens tillsammans bygger upp
// korrekta kommandon.
func Parser(tokens <-chan Token, reps chan<- []Command, wait_toks *sync.WaitGroup, wait_send *sync.WaitGroup, erow chan<- int) {
	var last_row int
	var cit_count int
	var next Command
	var cur *Command = &next
	var prev interface{} = Dot{}
	for tokenstruct := range tokens {
		token := tokenstruct.tok
		last_row = tokenstruct.row
		switch token := token.(type) {
		case Word:
			switch prev.(type) {
			case Dot: // DOT -> CMD
				next = InsertWord(next,tokenstruct,erow)
			case Cit: // CIT -> CMD
				next = InsertWord(next,tokenstruct,erow)
			case Int: // INT -> CMD
				if next.name == "REP" {
					// Sätter cur ett steg lägre för ny lista
					leaf := Command{next.name,next.arg,[]Command{},cur,true}
					(*cur).list = append((*cur).list,leaf)
					cur = &(*cur).list[len((*cur).list)-1]
					next = InsertWord(next,tokenstruct,erow)
				} else {
					erow <- tokenstruct.row
				}
			default:
				erow <- tokenstruct.row
			}
		case Int: // FORW|BACK|LEFT|RIGHT|REP -> INT
			if cmd,b := prev.(Word); b {
				if _,b := cmd.word.(IntWord); b {
					next.arg = token.val
				} else {
					erow <- tokenstruct.row
				}
			} else {
				erow <- tokenstruct.row
			}
		case Color: // COLOR -> COL
			if cmd,b := prev.(Word); b {
				if _,b := cmd.word.(ColWord); b {
					next.arg = token.val
				} else {
					erow <- tokenstruct.row
				}
			} else {
				erow <- tokenstruct.row
			}
		case Dot:
			switch prev := prev.(type) {
			case Word: // DOWN|UP -> DOT
				if _,b := prev.word.(DotWord); b {
					cur = EndCommand(&next,cur,tokenstruct,reps,wait_send,erow)
				} else {
					erow <- tokenstruct.row
				}
			case Int: // INT -> DOT
				cur = EndCommand(&next,cur,tokenstruct,reps,wait_send,erow)
			case Color: // COL -> DOT
				cur = EndCommand(&next,cur,tokenstruct,reps,wait_send,erow)
			default:
				erow <- tokenstruct.row
			}
		case Cit:
			switch prev.(type) {
			case Int: // INT -> CIT
				if next.name == "REP" { 
					// Sätter cur ett steg lägre för ny lista
					leaf := Command{next.name,next.arg,[]Command{},cur,false}
					(*cur).list = append((*cur).list,leaf)
					cur = &(*cur).list[len((*cur).list)-1]
					cit_count++
				} else {
					erow <- tokenstruct.row
				}
			case Dot: // DOT -> CIT - end citation
				if cit_count != 0 { 
					cit_count--
					// sätt nytt mål ett steg högre
					cur = (*cur).back
					cur = ExitRep(&next,cur,reps,wait_send)
				} else {
					erow <- tokenstruct.row
				}
			case Cit: // CIT -> CIT - end citation
				if (cit_count != 0) && (next.name != "REP") { 
					cit_count--
					// sätt nytt mål ett steg högre
					cur = (*cur).back
					cur = ExitRep(&next,cur,reps,wait_send)
				} else {
					erow <- tokenstruct.row
				}
			default:
				erow <- tokenstruct.row
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

// Hjälpfunktion till Parser för att sätta in ett Ord i en repetitionslista
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

// Vad som händer när token 'punkt' läses av Parsern.
func EndCommand(source *Command, target *Command, tokenstruct Token, reps chan<- []Command, wg *sync.WaitGroup, erow chan<- int) *Command {
	if _,b := tokenstruct.tok.(Dot); b {
		if len((*source).list) == 0 {
			wg.Add(1)
			reps <- []Command{*source}
			*source = Command{}
		} else {
			(*target).list = append((*target).list,Command{source.name,source.arg,[]Command{},target,false})
			target = ExitRep(source,target,reps,wg)
		}
	} else {
		erow <- tokenstruct.row
	}
	return target
}

// Hjälpfunktion till Parser som klättrar upp ur repetitionsträd så länge
// repetitionen saknar citationstecken
func ExitRep(source *Command,target *Command,reps chan<- []Command, wg *sync.WaitGroup) *Command {
	for (*target).nocit {
		target = (*target).back
	}
	if source == target {
		wg.Add(1)
		reps <- (*source).list
		*source = Command{}
	}
	return target
}

// Tråd för att sända repetitioner (och vanliga kommandon) till Executor
func Sender(reps <-chan []Command, cmds chan<- Command, wait_send *sync.WaitGroup, wait_exec *sync.WaitGroup) {
	for list := range reps {
		Repeat(list,cmds,wait_exec)
		wait_send.Done()
	}
}

// Repeterar en lista av kommandon r antal gånger - kan anropa sig själv
// om det stöter på en ny repetition bland kommandona.
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


// Tråd för exekvering av kommandon, utför självaste sköldpaddekontrollen
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

// Hjälpfunktion till Executor, konverterar från float till string
func FloatToString(convert float64) string {
    // to convert a float number to a string
    return strconv.FormatFloat(convert, 'f', 6, 64)
}

// Hjälpfunktion till Executor, konverterar från string till float
func StringToFloat(convert string) float64 {
     if n, err:= strconv.ParseFloat(convert, 64); err == nil {
            return n
     } else {
     	//TODO error message
     	return 0.0

     }
}

// Hjälpfunktion till Executor, konverterar från string till int
func StringToInt(convert string) int {
	if r, err:= strconv.Atoi(convert);err==nil{
		return r
	} else {
		//TODO Error message
		return 0	
	}
}
