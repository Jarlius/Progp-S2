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
type RepWord struct {val string}

type Color struct {val string}
type Int struct {val string}

type Dot struct {}
type Cit struct {}

type Command struct {
	name string
	arg string
	rep *Command
	next *Command
}

// Mainfunktion, skapar kanaler, trådar och skriver ut slutresultat
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	input := make(chan string)
	tokens := make(chan Token)
	reps := make(chan Command)
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
	iwordex,_ := regexp.Compile(`^(FORW|BACK|LEFT|RIGHT)$`)
	cwordex,_ := regexp.Compile(`^COLOR$`)
	dwordex,_ := regexp.Compile(`^(DOWN|UP)$`)
	rwordex,_ := regexp.Compile(`^REP$`)
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
				case rwordex.MatchString(trim):
					wait_toks.Add(1)
					tokens <- Token{row,Word{RepWord{trim}}}
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
func Parser(tokens <-chan Token, reps chan<- Command, wait_toks *sync.WaitGroup, wait_send *sync.WaitGroup, erow chan<- int) {
	for token := range tokens {
		cmd,_ := ParseCmd(token,tokens,wait_toks,erow)
		wait_send.Add(1)
		wait_toks.Done()
		reps <- cmd
	}
	erow <- 0
}

// Rekursiv metod för Parser
func ParseCmd(token Token, tokens <-chan Token, wait_toks *sync.WaitGroup, erow chan<- int) (Command,int) {
	if word,b := token.tok.(Word); b {
		wait_toks.Done()
		switch word := word.word.(type) {
		case IntWord:
			number := <-tokens
			if num,b := number.tok.(Int); b {
				wait_toks.Done()			
				dot := <-tokens
				if _,b := dot.tok.(Dot); b {
					return Command{name:word.val,arg:num.val,next:&(Command{})},dot.row
				} else {
					if dot.row != 0 {
						erow <- dot.row
					} else {
						erow <- number.row
					}
				}
			} else {
				if number.row != 0 {
					erow <- number.row
				} else {
					erow <- token.row
				}
			}
		case ColWord:
			color := <-tokens
			if col,b := color.tok.(Color); b {
				wait_toks.Done()
				dot := <-tokens
				if _,b := dot.tok.(Dot); b {
					return Command{name:word.val,arg:col.val,next:&(Command{})},dot.row
				} else {
					if dot.row != 0 {
						erow <- dot.row
					} else {
						erow <- color.row
					}
				}
			} else {
				if color.row != 0 {
					erow <- color.row
				} else {
					erow <- token.row
				}
			}
		case DotWord:
			dot := <-tokens
			if _,b := dot.tok.(Dot); b {
				return Command{name:word.val,next:&(Command{})},dot.row
			} else {
				if dot.row != 0 {
					erow <- dot.row
				} else {
					erow <- token.row
				}
			}			
		case RepWord:
			number := <-tokens
			if num,b := number.tok.(Int); b {
				wait_toks.Done()
				rep,end := ParseRep(number.row,tokens,wait_toks,erow)
				return Command{word.val,num.val,rep,&(Command{})},end
			} else {
				if number.row != 0 {
					erow <- number.row
				} else {
					erow <- token.row
				}
			}
		}
	} else {
		erow <- token.row
	}
	dummy := make(chan struct{})
	<-dummy
	return Command{},0
}

func ParseRep(prev int, tokens <-chan Token, wait_toks *sync.WaitGroup, erow chan<- int) (*Command,int) {
	citorcmd := <-tokens
	if _,b := citorcmd.tok.(Cit); b {
		wait_toks.Done()
		nextoken := <-tokens
		if _,b := nextoken.tok.(Word); b {
			rep,end,last := ParseExp(nextoken,tokens,wait_toks,erow)
			if _,b := end.tok.(Cit); b {
				return &rep,end.row
			} else {
				if end.row != 0 {
					erow <- end.row
				} else {
					erow <- last
				}
			}
		} else {
			erow <- nextoken.row
		}
	} else if _,b := citorcmd.tok.(Word); b {
		repcommand,end := ParseCmd(citorcmd,tokens,wait_toks,erow)
		return &repcommand,end
	} else {
		if citorcmd.row != 0 {
			erow <- citorcmd.row
		} else {
			erow <- prev
		}
	}
	dummy := make(chan struct{})
	<-dummy
	return &(Command{}),0
}

func ParseExp(word Token, tokens <-chan Token, wait_toks *sync.WaitGroup, erow chan<- int) (Command,Token,int) {
	cmd,last := ParseCmd(word,tokens,wait_toks,erow)
	wait_toks.Done()
	nextword := <-tokens
	if _,b := nextword.tok.(Word); b {
		next,end,last := ParseExp(nextword,tokens,wait_toks,erow)
		cmd.next = &next
		return cmd,end,last
	}
	return cmd,nextword,last
}

// Tråd för att sända repetitioner (och vanliga kommandon) till Executor
func Sender(reps <-chan Command, cmds chan<- Command, wait_send *sync.WaitGroup, wait_exec *sync.WaitGroup) {
	for cmd := range reps {
		Repeat(cmd,cmds,wait_exec)
		wait_send.Done()
	}
}

// Repeterar en lista av kommandon r antal gånger - kan anropa sig själv
// om det stöter på en ny repetition bland kommandona.
func Repeat(cmd Command, cmds chan<- Command, wait_exec *sync.WaitGroup) {
	for ; cmd != (Command{}); cmd = *(cmd.next) {
		if cmd.name == "REP" {
			reps,_ := strconv.Atoi(cmd.arg)
			for i := 0; i < reps; i++ {
				Repeat(*(cmd.rep),cmds,wait_exec)
			}
		} else {
			wait_exec.Add(1)
			cmds <- cmd
		}
	}
}

// Tråd för exekvering av kommandon, utför självaste sköldpaddekontrollen
func Executor(commands <-chan Command, wg *sync.WaitGroup, output chan<- string) {
	var answer []string
	var down bool
	color := "#0000FF"
	var rotation float64
	var X float64
	var Y float64
	for command := range commands {
		switch command.name {
		case "DOWN":
			down = true
		case "UP":
			down = false
		case "FORW":
			X2,Y2 := Movement(rotation,command.arg)
			if down {
				X1 := X
				Y1 := Y
				X += X2
				Y += Y2
				AddToAnswer(color,X1,Y1,X,Y,&answer)
			} else {
				X += X2
				Y += Y2
			}
		case "BACK":
			X2,Y2 := Movement(rotation,command.arg)
			if down {
				X1 := X
				Y1 := Y
				X -= X2
				Y -= Y2
				AddToAnswer(color,X1,Y1,X,Y,&answer)
			} else {
				X -= X2
				Y -= Y2
			}
		case "LEFT":
			rotation += DegToRad(command.arg)
		case "RIGHT":
			rotation -= DegToRad(command.arg)
		case "COLOR":
			color = command.arg
		}
		wg.Done() 
	}
	output <- strings.Join(answer," ")
}

// Ny position efter rörelse, fram eller bak
func Movement(rotation float64, arg string) (float64,float64) {
	n,_ := strconv.ParseFloat(arg, 64)
	X := math.Cos(rotation)*n
	Y := math.Sin(rotation)*n
	return X,Y
}

// Lägg till en linje till svarslistan
func AddToAnswer(color string, X1 float64, Y1 float64, X2 float64, Y2 float64, answer *[]string) {
	newans := []string{
		color,
		strconv.FormatFloat(X1, 'f', 4, 64),
		strconv.FormatFloat(Y1, 'f', 4, 64),
		strconv.FormatFloat(X2, 'f', 4, 64),
		strconv.FormatFloat(Y2, 'f', 4, 64),
		"\n",
	}
	*answer = append(*answer, newans...)
}

// Ändra från grader till radianer
func DegToRad(arg string) float64 {
	degs,_ := strconv.Atoi(arg)
	return float64(degs)*math.Pi/180
}

