// Laboration S2 av Jarl Silvén och Simon Hellberg
package main

import (
	"bufio"
	"os"
	"sync"
	"unicode"
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
	stopgex,_ := regexp.Compile(`^[\s\.%"]$`) 
	colorex,_ := regexp.Compile(`^\#[A-Z\d]{6}$`)
	integex,_ := regexp.Compile(`^[1-9]\d*$`)
	row := 1
	for words := range input {
		words = words + " "
		word := []byte{}
		for _,runa := range words {
			if len(word) == 0 && unicode.IsSpace(runa) {continue}
			if !stopgex.MatchString(string(runa)) {
				if unicode.IsLower(runa) {
					runa = unicode.ToUpper(runa)
				}
				word = append(word,byte(runa))
			} else {
				var token Token
				switch ord := string(word); {
				case ord == "FORW" || ord == "BACK" || ord == "LEFT" || ord == "RIGHT":
					token = Token{row,Word{IntWord{ord}}}
				case ord == "COLOR":
					token = Token{row,Word{ColWord{ord}}}
				case ord == "DOWN" || ord == "UP":
					token = Token{row,Word{DotWord{ord}}}
				case ord == "REP":
					token = Token{row,Word{RepWord{ord}}}
				case integex.Match(word):
					token = Token{row,Int{ord}}
				case colorex.Match(word):
					token = Token{row,Color{ord}}
				case len(word) == 0:
				default:
					erow <- row
				}
				if token != (Token{}) {
					wait_toks.Add(1)
					tokens <- token
				}
				if runa == '.' {
					wait_toks.Add(1)
					tokens <- Token{row,Dot{}}
				} else if runa == '"' {
					if len(word) == 0 {
						wait_toks.Add(1)
						tokens <- Token{row,Cit{}}
					} else {
						erow <- row
					}
				} else if runa == '%' {
					break;
				}
				word = []byte{}
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
		cmd,row := ParseCmd(token,tokens,wait_toks)
		if cmd != (Command{}) {
			wait_send.Add(1)
			wait_toks.Done()
			reps <- cmd
		} else {
			erow <- row
		}
	}
	erow <- 0
}

// Kontrollerar att ett kommando är semantiskt korrekt. Tar emot första token
// som man kan vilja peek-a på i anroparen, men hämtar resten av tokens från kanal
// Returnerar kommando och radnummer. Är kommandot tomt används rad till errormeddelande
func ParseCmd(token Token, tokens <-chan Token, wait_toks *sync.WaitGroup) (Command,int) {
	var prev int
	if word,b := token.tok.(Word); b {
		wait_toks.Done()
		prev = token.row
		switch word := word.word.(type) {
		case IntWord:
			token = <-tokens
			if num,b := token.tok.(Int); b {
				wait_toks.Done()			
				prev = token.row
				token = <-tokens
				if _,b := token.tok.(Dot); b {
					return Command{name:word.val,arg:num.val,next:&(Command{})},token.row
				}
			}
		case ColWord:
			token = <-tokens
			if col,b := token.tok.(Color); b {
				wait_toks.Done()
				prev = token.row
				token = <-tokens
				if _,b := token.tok.(Dot); b {
					return Command{name:word.val,arg:col.val,next:&(Command{})},token.row
				}
			}
		case DotWord:
			token = <-tokens
			if _,b := token.tok.(Dot); b {
				return Command{name:word.val,next:&(Command{})},token.row
			}
		case RepWord:
			token = <-tokens
			if num,b := token.tok.(Int); b {
				wait_toks.Done()
				rep,end := ParseRep(token.row,tokens,wait_toks)
				if *rep != (Command{}) {
					return Command{word.val,num.val,rep,&(Command{})},end
				} else {
					return *rep,end
				}
			}
		}
	}
	if token.row != 0 {
		return (Command{}),token.row
	} else {
		return (Command{}),prev
	}
}

// Kontrollerar att en repetition är semantiskt korrekt. Tar emot radnummret på anroparens token och
// returnerar pekare för repetitionskommandot att repetera. Tom pekare betyder error på returrad.
func ParseRep(prev int, tokens <-chan Token, wait_toks *sync.WaitGroup) (*Command,int) {
	token := <-tokens
	if _,b := token.tok.(Cit); b {
		wait_toks.Done()
		prev = token.row
		token = <-tokens
		if _,b := token.tok.(Word); b {
			var rep Command
			rep,token,prev = ParseExp(token,tokens,wait_toks)
			if _,b := token.tok.(Cit); b {
				return &rep,token.row
			}
		}
	} else if _,b := token.tok.(Word); b {
		cmd,end := ParseCmd(token,tokens,wait_toks)
		return &cmd,end
	}
	if token.row != 0 {
		return &(Command{}),token.row
	} else {
		return &(Command{}),prev
	}
}

// Skapar en uttryck av kommandon. Tar emot Token som garanterat är ett ord, och returnerar första 
// kommandot i kedjan tillsammans med det token som avbröt kedjan och radnummret på sista kommandot
func ParseExp(word Token, tokens <-chan Token, wait_toks *sync.WaitGroup) (Command,Token,int) {
	cmd,prev := ParseCmd(word,tokens,wait_toks)
	wait_toks.Done()
	nextword := <-tokens
	if _,b := nextword.tok.(Word); b {
		next,token,prev := ParseExp(nextword,tokens,wait_toks)
		cmd.next = &next
		return cmd,token,prev
	}
	return cmd,nextword,prev
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

